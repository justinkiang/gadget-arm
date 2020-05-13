package session

import (
	"log"
	"os"
	"strings"
	"sync"

	"crypto/tls"
	"crypto/x509"
	"github.com/justinkiang/gadget-arm/errors"
	"gopkg.in/mgo.v2"
	"net"
	"time"
)

var sessions = make(map[string]*mgo.Session)
var mutex = &sync.Mutex{}

func Get(connectionVariable string, cert ...string) *mgo.Session {
	if sessions[connectionVariable] == nil {
		mutex.Lock()
		defer mutex.Unlock()
		if sessions[connectionVariable] == nil {
			var cs string

			if strings.HasPrefix(connectionVariable, "mongodb://") {
				cs = connectionVariable
			} else {
				cs = os.Getenv(connectionVariable)
			}

			var err error

			var session *mgo.Session
			if cert != nil {
				session, err = dialWithSSL(cs, cert[0])
			} else if strings.Contains(cs, "&ssl=true") || strings.Contains(cs, "?ssl=true") {
				cs = strings.ReplaceAll(cs, "&ssl=true", "")
				cs = strings.ReplaceAll(cs, "?ssl=true", "")
				dialInfo, parseErr := mgo.ParseURL(cs)
				if parseErr != nil {
					log.Panicf("unable to parse connection string, %s", parseErr.Error())
				}

				//Below part is similar to above.
				tlsConfig := &tls.Config{}
				dialInfo.DialServer = func(addr *mgo.ServerAddr) (net.Conn, error) {
					conn, err := tls.Dial("tcp", addr.String(), tlsConfig)
					return conn, err
				}
				session, err = mgo.DialWithInfo(dialInfo)
			} else {
				session, err = mgo.Dial(cs)
			}

			errors.Check(err)

			session.SetSocketTimeout(10 * time.Second)
			session.SetSyncTimeout(10 * time.Second)

			// http://godoc.org/labix.org/v2/mgo#Session.SetMode
			session.SetMode(mgo.Monotonic, true)
			sessions[connectionVariable] = session
		}
	} else {
		sessions[connectionVariable].Refresh()
	}

	return sessions[connectionVariable].Copy()
}

func dialWithSSL(cs, pem string) (session *mgo.Session, err error) {
	roots := x509.NewCertPool()
	roots.AppendCertsFromPEM([]byte(pem))

	tlsConfig := &tls.Config{}
	tlsConfig.RootCAs = roots

	dialInfo, err := mgo.ParseURL(cs)
	if err != nil {
		return
	}
	dialInfo.DialServer = func(addr *mgo.ServerAddr) (net.Conn, error) {
		conn, err := tls.Dial("tcp", addr.String(), tlsConfig)
		return conn, err
	}
	session, err = mgo.DialWithInfo(dialInfo)
	return
}
