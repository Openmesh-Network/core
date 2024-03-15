package database

import (
    "context"
    "database/sql"
    "fmt"
    _ "github.com/lib/pq"
    "openmesh.network/openmesh-core/internal/config"
    "openmesh.network/openmesh-core/internal/logger"
    "time"
)

const (
    SQL_NODE_META_EXISTS   = `SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = $1)`
    SQL_TRUNCATE_NODE_META = `TRUNCATE TABLE nodemeta;`
    SQL_CREATE_NODE_META   = `CREATE TABLE IF NOT EXISTS nodemeta (
    id SERIAL PRIMARY KEY,
    datasource VARCHAR(255) NOT NULL,
    tablename VARCHAR(255) NOT NULL
);`
    SQL_ADD_NODE_META = `INSERT INTO nodemeta (datasource, tablename) VALUES ($1, $2)`
)

// Instance is the instance that holds the database connection
type Instance struct {
    Conn *sql.DB
}

func NewInstance() (*Instance, error) {
    i := &Instance{}
    conf := config.Config.DB
    connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
        conf.URL, conf.Port, conf.Username, conf.Password, conf.DBName)

    // Initialise PostgreSQL connection
    conn, err := sql.Open("postgres", connStr)
    if err != nil {
        return nil, err
    }
    i.Conn = conn

    // Check if the connection is successful
    timeoutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    if err := conn.PingContext(timeoutCtx); err != nil {
        defer conn.Close()
        return nil, err
    }
    i.Conn = conn
    logger.Infof("Successfully connected to PostgreSQL at %s@%s:%d", conf.DBName, conf.URL, conf.Port)

    return i, nil
}

// Start establish the connection to PostgreSQL and fill in the Conn field in the instance
func (i *Instance) Start() error {
    // Create nodemeta table if it not exists
    if err := i.CreateNodeMetaTable(); err != nil {
        defer i.Conn.Close()
        return err
    }
    logger.Debugf("Successfully created table 'nodemeta' or it already exists")

    return nil
}

// Stop closes the connection to the remote host
func (i *Instance) Stop() error {
    if i.Conn != nil {
        return i.Conn.Close()
    }
    return nil
}

// CreateNodeMetaTable truncates or creates table "nodemeta"
func (i *Instance) CreateNodeMetaTable() error {
    // Check if table "nodemeta" exists
    var tableExists bool
    err := i.Conn.QueryRow(SQL_NODE_META_EXISTS, "nodemeta").Scan(&tableExists)
    if err != nil {
        // Handle error
        return err
    }

    // Truncate this table if it exists
    if tableExists {
        _, err := i.Conn.Exec(SQL_TRUNCATE_NODE_META)
        if err != nil {
            // Handle error
            return err
        }
    }

    // Create a new table "nodemeta"
    _, err = i.Conn.Exec(SQL_CREATE_NODE_META)
    return err
}

// AddNodeMeta insert a new data source into the specific table
func (i *Instance) AddNodeMeta(dataSource, table string) error {
    _, err := i.Conn.Exec(SQL_ADD_NODE_META, dataSource, table)
    return err
}
