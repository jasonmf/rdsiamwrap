# rdsiamwrap

AWS allows you to authenticate to an RDS database using IAM. Configuration information can be found in [RDS documentation](https://aws.amazon.com/premiumsupport/knowledge-center/users-connect-rds-iam/). The Go SDK has a [package](https://godoc.org/github.com/aws/aws-sdk-go/service/rds/rdsutils) which helps with creating the connection.

This is great for brief database activity. However, if a new connection has to be established the Go `sql` package will reuse the existing DSN to create that connection. This DSN will always contain the same authentication token which will have expired after 15 minutes.

The `rdsiamwrap` package will allow you to create a custom `Driver` that creates the authentication token and rebuilds the DSN _on demand_. If the token it has is still valid it reuses that. If it has exceeded a configurable lifetime (default is 14 minutes) then a new authentication token and DSN are generated and used.

## Usage

```
package main

import (
        "crypto/tls"
        "crypto/x509"
        "database/sql"
        "log"
        "net/url"

        "github.com/AgentZombie/rdsiamwrap"
        "github.com/aws/aws-sdk-go/aws/credentials/ec2rolecreds"
        "github.com/aws/aws-sdk-go/aws/session"
        "github.com/go-sql-driver/mysql"
)

const DriverName = "mydriver"
const TLSCfgName = "rds"

// CA Chain from https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/UsingWithRDS.SSL.html
const RDSCAChain = `-----BEGIN CERTIFICATE-----
MIID9DCCAtygAwIBAgIBQjANBgkqhkiG9w0BAQUFADCBijELMAkGA1UEBhMCVVMx
...
-----END CERTIFICATE-----`

func main() {
        rootCertPool := x509.NewCertPool()
        if ok := rootCertPool.AppendCertsFromPEM([]byte(RDSCAChain)); !ok {
                panic("failed to parse RDS CA chain")
        }
        mysql.RegisterTLSConfig(TLSCfgName, &tls.Config{
                RootCAs: rootCertPool,
        })

        v := url.Values{}
        v.Add("tls", TLSCfgName)
        v.Add("allowCleartextPasswords", "true")
        v.Add("parseTime", "true")

        d := rdsiamwrap.New()
        d.Driver = &mysql.MySQLDriver{}
        d.Addr = "db-cluster.cluster-potatochip.us-west-2.rds.amazonaws.com:3306"
        d.Region = "us-west-2"
        d.User = "myrole"
        d.DBName = "mydb"
        // Use the role associated with this EC2 instance
        d.Creds = ec2rolecreds.NewCredentials(session.Must(session.NewSession()))
        d.Params = v
        sql.Register(DriverName, d)

        pool, err := sql.Open(DriverName, "this DSN is unused")
        if err != nil {
                log.Fatal("unable to use data source name", err)
        }
        defer pool.Close()
}

```

## Limitations

- You _must_ register an `rdsiamwrap.Driver` and do so only once.
- You _must_ be able to get an instance of the `database/sql/driver.Driver` you want to wrap.
