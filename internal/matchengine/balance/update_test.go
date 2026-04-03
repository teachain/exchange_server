package balance

import (
	"testing"
)

func TestUpdateManager_key(t *testing.T) {
	m := &UpdateManager{}
	result := m.key(123, "BTC", "trade", "456")
	expected := "update:123:BTC:trade:456"
	if result != expected {
		t.Errorf("expected %s, got %s", expected, result)
	}
}

func TestUpdateManager_key_zeroUserID(t *testing.T) {
	m := &UpdateManager{}
	result := m.key(0, "ETH", "fee", "789")
	expected := "update:0:ETH:fee:789"
	if result != expected {
		t.Errorf("expected %s, got %s", expected, result)
	}
}
