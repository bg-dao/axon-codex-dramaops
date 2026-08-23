package secret

import (
	"errors"
	"testing"
)

func TestResolveVoiceBindingRequiresDeviceConsent(t *testing.T) {
	store := NewMemoryStore()
	if err := store.Set(VoiceBindingEntry("voice-1"), "provider-voice"); err != nil {
		t.Fatal(err)
	}
	if _, err := ResolveVoiceBinding(store, "voice-1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("voice without consent = %v", err)
	}
	if err := store.Set(VoiceConsentEntry("voice-1"), "provider-consent"); err != nil {
		t.Fatal(err)
	}
	value, err := ResolveVoiceBinding(store, "voice-1")
	if err != nil || value != "provider-voice" {
		t.Fatalf("binding = %q, err = %v", value, err)
	}
}
