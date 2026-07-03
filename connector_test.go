package golem

import "testing"

type fakeConnector struct{}

func (fakeConnector) Connect() (Dialect, error) { return fakeDialect{}, nil }
func (fakeConnector) Close() error              { return nil }

var _ Connector = (*fakeConnector)(nil)

func TestFakeConnector_Connect(t *testing.T) {
	c := fakeConnector{}
	d, err := c.Connect()
	if err != nil {
		t.Fatalf("Connect returned error: %v", err)
	}
	if d == nil {
		t.Fatal("Connect returned nil Dialect")
	}
}

func TestFakeConnector_Close(t *testing.T) {
	c := fakeConnector{}
	if err := c.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}
}
