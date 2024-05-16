package database

import (
	"fmt"
	"testing"
)

func TestAddMessages(t *testing.T) {
	// Initialise the DB connection
	SqlDB, err := NewSqlInstance()
	if err != nil {
		panic(err)
	}
	defer SqlDB.Conn.Close()

	// Try some basic JSON reading.
	easyTestMsg := `{"foo":{"John":{"Doe":"bar"}},"dee":{"Doo":"daa"}}`
	SqlDB.addMessages(easyTestMsg)

	sqlrows, err := SqlDB.Conn.Query("SELECT * FROM openmesh_data")
	var datastring string
	t.Log(sqlrows.Scan(datastring))
	if err != nil {
		t.Log(err)
		panic(err)
	}
	fmt.Printf("sqlrows: %v\n", sqlrows)

}

func TestCollectionAddMessages(t *testing.T) {
	// Initialise the DB connection
	SqlDB, err := NewSqlInstance()
	if err != nil {
		panic(err)
	}
	defer SqlDB.Conn.Close()

	// This function will read JSON messages from a supported Source (See collector package) and try to add them to the SQL database.

}
