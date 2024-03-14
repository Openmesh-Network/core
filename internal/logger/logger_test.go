package logger

import "testing"

func TestInitLogger(t *testing.T) {
    // This initialises a production logger and print JSON-styled logs in the console
    InitLogger()
    Infof("This is test logger: %s", "INFO level")
    Errorf("This is also test logger: %s", "ERROR level")
}
