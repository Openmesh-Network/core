# Internal

## Development Guide for Private Libraries

Step 1: Initialise an `Instance`:

```go
package xxx

type Instance struct {
    dependency otherlib.Instance
    // Some arbitrary properties......
}
```

Step 2: Add a `NewInstance()` function for initialisation. If you need to use some other libraries (like `database.Instance` for accessing data stored in PostgreSQL), you can pass it to this instance in this function.

```go
func NewInstance(otherIns *otherlib.Instance) *Instance {
    // Some arbitrary initialisation progress......
    return &Instance{dependency: otherIns}
}
```

Step 3: Call `xxx.NewInstance()` in `main()` and pass the potential dependencies into it.

```go
otherIns := otherlib.NewInstance()
ins := xxx.NewInstance(otherIns)
```

Please see `internal/core/instance.go` and `main.go` for reference.
