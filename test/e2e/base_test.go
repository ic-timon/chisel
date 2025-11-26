package e2e_test

import (
	"testing"
	"time"

	chclient "github.com/jpillora/chisel/client"
	chserver "github.com/jpillora/chisel/server"
)

func TestBase(t *testing.T) {
	tmpPort := availablePort()
	//setup server, client, fileserver
	teardown := simpleSetup(t,
		&chserver.Config{},
		&chclient.Config{
			Remotes: []string{tmpPort + ":$FILEPORT"},
		})
	defer teardown()
	//test remote
	result, err := post("http://localhost:"+tmpPort, "foo")
	if err != nil {
		t.Fatal(err)
	}
	if result != "foo!" {
		t.Fatalf("expected exclamation mark added")
	}
}

func TestReverse(t *testing.T) {
	tmpPort := availablePort()
	//setup server, client, fileserver
	teardown := simpleSetup(t,
		&chserver.Config{
			Reverse: true,
		},
		&chclient.Config{
			Remotes: []string{"R:127.0.0.1:" + tmpPort + ":127.0.0.1:$FILEPORT"},
		})
	defer teardown()
	// Wait a bit for connections to stabilize
	time.Sleep(100 * time.Millisecond)
	//test remote (this goes through the server and out the client)
	result, err := post("http://localhost:"+tmpPort, "foo")
	if err != nil {
		t.Fatal(err)
	}
	if result != "foo!" {
		t.Fatalf("expected exclamation mark added")
	}
}
