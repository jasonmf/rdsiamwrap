// rdsiamwrap wraps a database/sql/driver.Driver to provide AWS authentication to RDS databases. Doing this authentication using the rdsutils package alone works for an initial connection but fails if new connections need to be established but the token has expired. This package generates this token as needed using provided AWS credentials.
package rdsiamwrap

import (
	"database/sql/driver"
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/service/rds/rdsutils"
)

const (
	// Get a new token after this duration if none is specified in the driver struct.
	// AWS's default lifetime is 15 minutes.
	DefaultTokenLifetime = time.Minute * 14 // one minute under 15 minute documented expiration
)

// Driver implements the sql.driver.Driver interface to provide AWS IAM authentication around an existing driver.
type Driver struct {
	// The actual driver to be wrapped
	Driver driver.Driver
	// Address of the RDS instance
	Addr string
	// AWS region of the RDS instance
	Region string
	// Database username: should be the role name
	User string
	// Name of the database
	DBName string
	// AWS credentials for authentication
	Creds *credentials.Credentials
	// Parameters to pass
	Params url.Values
	// How long to use an auth token. If unset DefaultTokenLifetime is used
	TokenLifetime time.Duration

	lastDSN    string
	lock       *sync.Mutex
	renewAfter time.Time
}

// New creates a new Driver which will still need configuration fields set.
func New() *Driver {
	d := &Driver{
		lock: &sync.Mutex{},
	}
	return d
}

// Open opens a new database connection. It creates a new auth token as needed, reusing one that hasn't expired yet.
func (d *Driver) Open(name string) (driver.Conn, error) {
	d.lock.Lock()
	defer d.lock.Unlock()
	now := time.Now()
	if now.After(d.renewAfter) {
		b := rdsutils.NewConnectionStringBuilder(d.Addr, d.Region, d.User, d.DBName, d.Creds)
		connectStr, err := b.WithTCPFormat().WithParams(d.Params).Build()
		if err != nil {
			return nil, fmt.Errorf("building connection string: %w", err)
		}
		d.lastDSN = connectStr
		lifetime := DefaultTokenLifetime
		if d.TokenLifetime != 0 {
			lifetime = d.TokenLifetime
		}
		d.renewAfter = now.Add(lifetime)
	}
	return d.Driver.Open(d.lastDSN)
}
