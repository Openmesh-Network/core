# Networking Packages

These are packages supporting both the inter-node networking and the Xnode overlay network.

## p2p directory

This is the library implemented P2P networking based on `libp2p`.

### Instance Initialisation

```go
// Construct and start a p2p instance
ins, err := p2p.NewInstance(cancelContext).SetP2PHost(existingHost).Build()
err = ins.Start()
```

### Gossip Publish-and-Subscribe (pub-sub)

```go
// Join the specific topic
err := ins.Join("topic")

// Publisher sample usage
err = ins.Publish("topic", []byte("message"))

// Subscriber sample usage
msg, err := s.Subscribe(topic)
go func() {
    for {
        select {
        case <-cancelContext.Done():
            return
        case m := <-msg:
            // Handle message......
        }
    }
}()
```

### DHT (Distributed Hash Table)

```go
// Add a key to DHT
err := ins.DHT.PutValue(timeoutCtx, "key", []byte("value"))

// Get a value from DHT
value, err := ins.DHT.GetValue(timeoutCtx, "key")
```
