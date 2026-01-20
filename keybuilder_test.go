package cache

import (
	"testing"
)

func TestKeyBuilder_Light(t *testing.T) {
	kb := NewKeyBuilder()
	got := kb.Light("abc-123")
	want := "light:abc-123"

	if got != want {
		t.Errorf("Light() = %v, want %v", got, want)
	}
}

func TestKeyBuilder_Room(t *testing.T) {
	kb := NewKeyBuilder()
	got := kb.Room("room-id")
	want := "room:room-id"

	if got != want {
		t.Errorf("Room() = %v, want %v", got, want)
	}
}

func TestKeyBuilder_Zone(t *testing.T) {
	kb := NewKeyBuilder()
	got := kb.Zone("zone-id")
	want := "zone:zone-id"

	if got != want {
		t.Errorf("Zone() = %v, want %v", got, want)
	}
}

func TestKeyBuilder_Scene(t *testing.T) {
	kb := NewKeyBuilder()
	got := kb.Scene("scene-id")
	want := "scene:scene-id"

	if got != want {
		t.Errorf("Scene() = %v, want %v", got, want)
	}
}

func TestKeyBuilder_SmartScene(t *testing.T) {
	kb := NewKeyBuilder()
	got := kb.SmartScene("smart-id")
	want := "smart_scene:smart-id"

	if got != want {
		t.Errorf("SmartScene() = %v, want %v", got, want)
	}
}

func TestKeyBuilder_GroupedLight(t *testing.T) {
	kb := NewKeyBuilder()
	got := kb.GroupedLight("grouped-id")
	want := "grouped_light:grouped-id"

	if got != want {
		t.Errorf("GroupedLight() = %v, want %v", got, want)
	}
}

func TestKeyBuilder_Device(t *testing.T) {
	kb := NewKeyBuilder()
	got := kb.Device("device-id")
	want := "device:device-id"

	if got != want {
		t.Errorf("Device() = %v, want %v", got, want)
	}
}

func TestKeyBuilder_Bridge(t *testing.T) {
	kb := NewKeyBuilder()
	got := kb.Bridge("bridge-id")
	want := "bridge:bridge-id"

	if got != want {
		t.Errorf("Bridge() = %v, want %v", got, want)
	}
}

func TestKeyBuilder_BridgeHome(t *testing.T) {
	kb := NewKeyBuilder()
	got := kb.BridgeHome("home-id")
	want := "bridge_home:home-id"

	if got != want {
		t.Errorf("BridgeHome() = %v, want %v", got, want)
	}
}

func TestKeyBuilder_Resource(t *testing.T) {
	kb := NewKeyBuilder()
	got := kb.Resource("custom_type", "custom-id")
	want := "custom_type:custom-id"

	if got != want {
		t.Errorf("Resource() = %v, want %v", got, want)
	}
}

func TestKeyBuilder_Patterns(t *testing.T) {
	kb := NewKeyBuilder()

	tests := []struct {
		name string
		fn   func() string
		want string
	}{
		{
			name: "AllLights",
			fn:   kb.AllLights,
			want: "light:*",
		},
		{
			name: "AllRooms",
			fn:   kb.AllRooms,
			want: "room:*",
		},
		{
			name: "AllZones",
			fn:   kb.AllZones,
			want: "zone:*",
		},
		{
			name: "AllScenes",
			fn:   kb.AllScenes,
			want: "scene:*",
		},
		{
			name: "All",
			fn:   kb.All,
			want: "*",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.fn()
			if got != tt.want {
				t.Errorf("%s() = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestKeyBuilder_AllResources(t *testing.T) {
	kb := NewKeyBuilder()
	got := kb.AllResources("custom_type")
	want := "custom_type:*"

	if got != want {
		t.Errorf("AllResources() = %v, want %v", got, want)
	}
}
