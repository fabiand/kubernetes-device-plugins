package kvm

import (
	"os"
	"strconv"

	"github.com/golang/glog"
	"golang.org/x/net/context"
	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1alpha"

	"kubevirt.io/kubernetes-device-plugins/pkg/dpm"
)

const (
	KVMPath           = "/dev/kvm"
	KVMName           = "kvm"
	resourceNamespace = "devices.kubevirt.io/"
)

// KVMLister is the object responsible for discovering initial pool of devices and their allocation.
type KVMLister struct{}

// KVMDevicePlugin is an implementation of DevicePlugin that is capable of exposing /dev/kvm to containers.
type KVMDevicePlugin struct {
	dpm.DevicePlugin
	counter int
}

// Discovery discovers all KVM devices within the system.
func (kvm KVMLister) Discover() *dpm.DeviceMap {
	var devices = make(dpm.DeviceMap)

	if _, err := os.Stat(KVMPath); err == nil {
		glog.V(3).Infof("Discovered %s", KVMPath)
		devices["kvm"] = append(devices["kvm"], KVMName)
	}

	return &devices
}

// newDevicePlugin creates a DevicePlugin for specific deviceID, using deviceIDs as initial device "pool".
func (kvm KVMLister) NewDevicePlugin(deviceID string, deviceIDs []string) dpm.DevicePluginInterface {
	return dpm.DevicePluginInterface(newDevicePlugin(deviceID, deviceIDs))
}

// newDevicePlugin is a helper for NewDevicePlugin call. It has been separated to ease interface checking.
func newDevicePlugin(deviceID string, deviceIDs []string) *KVMDevicePlugin {
	var devs []*pluginapi.Device

	for _, deviceID := range deviceIDs {
		devs = append(devs, &pluginapi.Device{
			ID:     deviceID,
			Health: pluginapi.Healthy,
		})
	}

	glog.V(3).Infof("Creating device plugin %s, initial devices %v", deviceID, devs)
	ret := &KVMDevicePlugin{
		counter: 0,
	}
	ret.DevicePlugin = dpm.DevicePlugin{
		Socket:       pluginapi.DevicePluginPath + deviceID,
		Devs:         devs,
		ResourceName: resourceNamespace + deviceID,
		Update:       make(chan dpm.Message),
	}
	ret.DevicePlugin.Deps = ret

	return ret
}

// ListAndWatch sends gRPC stream of devices.
func (dpi *KVMDevicePlugin) ListAndWatch(e *pluginapi.Empty, s pluginapi.DevicePlugin_ListAndWatchServer) error {
	s.Send(&pluginapi.ListAndWatchResponse{Devices: dpi.DevicePlugin.Devs})

	for {
		select {
		case <-dpi.DevicePlugin.Update:
			s.Send(&pluginapi.ListAndWatchResponse{Devices: dpi.DevicePlugin.Devs})
		}
	}
}

// Allocate allocates a set of devices to be used by container runtime environment.
func (dpi *KVMDevicePlugin) Allocate(ctx context.Context, r *pluginapi.AllocateRequest) (*pluginapi.AllocateResponse, error) {
	var response pluginapi.AllocateResponse

	// This is super-hacky workaround for device quantity (that we don't care about in this case).
	dpi.DevicePlugin.Devs = append(dpi.DevicePlugin.Devs, &pluginapi.Device{
		ID:     KVMName + strconv.Itoa(dpi.counter),
		Health: pluginapi.Healthy,
	})
	dpi.counter += 1
	dpi.DevicePlugin.Update <- dpm.Message{}

	dev := new(pluginapi.DeviceSpec)
	dev.HostPath = "/dev/kvm"
	dev.ContainerPath = "/dev/kvm"
	dev.Permissions = "rw"
	response.Devices = append(response.Devices, dev)

	return &response, nil
}
