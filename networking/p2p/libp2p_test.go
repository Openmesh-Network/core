package p2p

import (
	"context"
	"testing"
	"time"

	"github.com/openmesh-network/core/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestPublish(t *testing.T) {
	config.ParseFlags()
	config.Path = "../../../"
	config.ParseConfig(config.Path, true)

	c, cancel := context.WithCancel(context.Background())
	defer cancel()
	p, err := NewInstance(c).Build()
	assert.NoError(t, err)
	err = p.Start()
	assert.NoError(t, err)

	s, err := NewInstance(c).Build()
	assert.NoError(t, err)
	err = s.Start()
	assert.NoError(t, err)

	topic := "xnode"

	// Wait for discover peers......
	time.Sleep(15 * time.Second)
	t.Log("15 seconds passed, try to pub-sub")

	err = p.JoinTopic(topic)
	assert.NoError(t, err)

	err = s.JoinTopic(topic)
	assert.NoError(t, err)

	msg, err := s.Subscribe(topic)
	go func() {
		for {
			select {
			case <-c.Done():
				return
			case m := <-msg:
				t.Logf("Got a message from pub-sub: %s", string(m.Data))
			}
		}
	}()

	err = p.Publish(topic, []byte("test123123"))
	assert.NoError(t, err)

	// Wait for receiving message
	time.Sleep(5 * time.Second)
}
