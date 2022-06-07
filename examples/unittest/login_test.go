package unittest

import (
	"github.com/stretchr/testify/assert"
	"github.com/sunrnalike/sun"
	"github.com/sunrnalike/sun/examples/dialer"
	"github.com/sunrnalike/sun/websocket"
	"testing"
	"time"
)

func login(account string) (sun.Client, error) {
	cli := websocket.NewClient(account, "unittest", websocket.ClientOptions{})

	cli.SetDialer(&dialer.ClientDialer{})
	err := cli.Connect("ws://localhost:8000")
	if err != nil {
		return nil, err
	}
	return cli, nil
}

func Test_login(t *testing.T) {
	cli, err := login("test1")
	assert.Nil(t, err)
	time.Sleep(time.Second * 3)
	cli.Close()
}
