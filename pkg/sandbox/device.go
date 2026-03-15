package sandbox

// DeviceType identifies the connection method for a device.
type DeviceType string

const (
	// DeviceLocal is the host machine (commands run via local shell).
	DeviceLocal DeviceType = "local"
	// DeviceADB is an Android device connected via ADB.
	DeviceADB DeviceType = "adb"
	// DeviceSSH is a remote machine reachable via SSH.
	DeviceSSH DeviceType = "ssh"
)

// Device represents a physical or virtual target in the device fleet.
// Each device has its own Executor, which the Fleet scheduler uses to
// dispatch commands to the correct target.
type Device struct {
	// ID is a unique identifier (e.g. "redmi-k50", "iphone-17", "ubuntu-srv-1").
	ID string

	// Name is a human-friendly display name.
	Name string

	// Type indicates the connection method.
	Type DeviceType

	// Address is the connection string:
	//   - local:  "" (ignored)
	//   - adb:    serial or "host:port" (e.g. "192.168.1.50:5555")
	//   - ssh:    "user@host" or "user@host:port"
	Address string

	// Executor is the sandbox backend bound to this device.
	// The Fleet scheduler injects this into each Engine's skill layer.
	Executor Executor

	// Tags are optional metadata for filtering/grouping (e.g. "android", "arm64").
	Tags []string
}

// DeviceFleet is a collection of devices that can be managed together.
type DeviceFleet struct {
	devices map[string]*Device
}

// NewDeviceFleet creates an empty fleet.
func NewDeviceFleet() *DeviceFleet {
	return &DeviceFleet{devices: make(map[string]*Device)}
}

// Add registers a device in the fleet. Panics on duplicate IDs.
func (f *DeviceFleet) Add(d *Device) {
	if _, exists := f.devices[d.ID]; exists {
		panic("duplicate device id: " + d.ID)
	}
	f.devices[d.ID] = d
}

// Get returns a device by ID, or nil if not found.
func (f *DeviceFleet) Get(id string) *Device {
	return f.devices[id]
}

// All returns all registered devices.
func (f *DeviceFleet) All() []*Device {
	out := make([]*Device, 0, len(f.devices))
	for _, d := range f.devices {
		out = append(out, d)
	}
	return out
}

// ByType returns all devices matching the given type.
func (f *DeviceFleet) ByType(t DeviceType) []*Device {
	var out []*Device
	for _, d := range f.devices {
		if d.Type == t {
			out = append(out, d)
		}
	}
	return out
}

// Count returns the number of registered devices.
func (f *DeviceFleet) Count() int {
	return len(f.devices)
}
