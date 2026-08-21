package supervisor

import "testing"

func TestConnUserFromChain(t *testing.T) {
	name, id := connUser("", "", []string{"user-7", "direct"})
	if id != 7 || name != "" {
		t.Fatalf("id chain: %q %d", name, id)
	}
	name, id = connUser("alice", "", nil)
	if name != "alice" || id != 0 {
		t.Fatalf("meta user: %q %d", name, id)
	}
	name, id = connUser("", "", []string{"user-bob"})
	if name != "bob" || id != 0 {
		t.Fatalf("name chain: %q %d", name, id)
	}
	name, id = connUser("", "", []string{"direct"})
	if name != "" || id != 0 {
		t.Fatalf("unattributed: %q %d", name, id)
	}
}
