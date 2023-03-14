package xds

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"

	bootstrapv3 "github.com/envoyproxy/go-control-plane/envoy/config/bootstrap/v3"
	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	"github.com/envoyproxy/go-control-plane/pkg/cache/types"
	"github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	"github.com/golang/protobuf/jsonpb"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	DefaultConfigKey = "envoy.json"
)

type ConfigClient interface {
	Get(ctx context.Context, ref apitypes.NamespacedName) (*bootstrapv3.Bootstrap, error)
	Set(ctx context.Context, ref apitypes.NamespacedName, cfg *bootstrapv3.Bootstrap) error
}

func ToMap(d *bootstrapv3.Bootstrap) map[resource.Type][]types.Resource {
	return map[resource.Type][]types.Resource{
		resource.ClusterType:  toResourceSlice(d.StaticResources.Clusters),
		resource.ListenerType: toResourceSlice(d.StaticResources.Listeners),
		resource.SecretType:   toResourceSlice(d.StaticResources.Secrets),
	}
}

func toResourceSlice[T types.Resource](s []T) []types.Resource {
	result := make([]types.Resource, len(s))
	for i, v := range s {
		result[i] = v
	}
	return result
}

type ConfigMapClient struct {
	Client client.Client
}

func NewConfigMapClient(client client.Client) *ConfigMapClient {
	return &ConfigMapClient{Client: client}
}

func (c *ConfigMapClient) Get(ctx context.Context, ref apitypes.NamespacedName) (*bootstrapv3.Bootstrap, error) {
	cm := &corev1.ConfigMap{}
	err := c.Client.Get(ctx, ref, cm)
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, err
	}

	if cm.Data == nil || len(cm.Data) == 0 {
		return &bootstrapv3.Bootstrap{StaticResources: &bootstrapv3.Bootstrap_StaticResources{}}, nil
	}

	// Read the first key
	var existingConfig string
	for _, existingConfig = range cm.Data {
		break
	}

	resp := &bootstrapv3.Bootstrap{}
	err = jsonpb.Unmarshal(strings.NewReader(existingConfig), resp)
	if err != nil {
		fmt.Println(existingConfig)
		return nil, fmt.Errorf("failed to unmarshal bootstrap config: %w", err)
	}
	return resp, nil
}

func (c *ConfigMapClient) Set(ctx context.Context, ref apitypes.NamespacedName, cfg *bootstrapv3.Bootstrap) error {
	if cfg.Node == nil {
		cfg.Node = &corev3.Node{}
	}
	if cfg.Node.Id == "" {
		return errors.New("node.id is required")
	}
	if cfg.Node.Cluster == "" {
		return errors.New("node.cluster is required")
	}

	buf := bytes.NewBuffer(nil)
	err := (&jsonpb.Marshaler{
		Indent: "  ",
	}).Marshal(buf, cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal bootstrap config: %w", err)
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ref.Name,
			Namespace: ref.Namespace,
		},
	}
	_, err = controllerutil.CreateOrUpdate(ctx, c.Client, cm, func() error {
		if cm.Data == nil {
			cm.Data = map[string]string{}
		}
		cm.Data[DefaultConfigKey] = buf.String()
		if cm.Labels == nil {
			cm.Labels = map[string]string{}
		}
		cm.Labels[LabelXDSKind] = ""

		return nil
	})
	return err
}
