package database

import (
	"database/sql"
	"fmt"
	"log"

	"github.com/Jeffail/gabs"
	_ "github.com/lib/pq" // Import PostgreSQL driver
)

// sqlInstance is the instance that holds the database connection for Pythia SQL queries (PostgreSQL)
type sqlInstance struct {
	Conn *sql.DB
}

// This will initiate a new connection to a local Postgres database. Make sure to call "defer <instance>.Conn.Close()" afterwards.
func NewSqlInstance() (*sqlInstance, error) {
	i := &sqlInstance{}
	db, err := sql.Open("postgres", "user=openmesh_core dbname=openmesh_db sslmode=disable") // No ssl for this user.
	if err != nil {
		return nil, err
	}

	// Return database as a connection in instance
	i.Conn = db

	// Create a table if none exists
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS openmesh_data (key TEXT, value TEXT)`)
	if err != nil {
		log.Fatal(err)
	}

	return i, nil
}

func (i *sqlInstance) addMessages(dynamicJSON string) {
	// Parse dynamic JSON using Gabs
	parsed, err := gabs.ParseJSON([]byte(dynamicJSON))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(parsed.String())

	// Get map of children from the parsed JSON
	childrenMap, err := parsed.ChildrenMap()
	if err != nil {
		log.Fatal(err)
	}

	// Iterate over the key-value pairs in the dynamic JSON
	for key, value := range childrenMap {
		// Insert each key-value pair into the SQLite table
		keyStr := key
		valueStr := value.String()

		_, err := i.Conn.Exec("INSERT INTO openmesh_data (key, value) VALUES (?, ?)", keyStr, valueStr) // This will certainly need to be refined.
		if err != nil {
			log.Println(err)
			continue
		}

		fmt.Printf("Inserted key: %s, value: %s\n", keyStr, valueStr)
	}

	fmt.Println("Dynamic JSON data inserted successfully!")
}
