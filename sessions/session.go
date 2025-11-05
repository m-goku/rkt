package sessions

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/alexedwards/scs/mysqlstore"
	"github.com/alexedwards/scs/postgresstore"
	"github.com/alexedwards/scs/redisstore"
	"github.com/alexedwards/scs/v2"
	"github.com/gomodule/redigo/redis"
)

type Session struct {
	CookieLifetime string
	CookiePersist  string
	CookieName     string
	CookieDomain   string
	SessionType    string
	CookieSecure   string
	DBPool         *sql.DB
	RedisPool      *redis.Pool
}

func (s *Session) InitSession() *scs.SessionManager {
	var persist, secure bool

	//how long should session last?
	minutes, err := strconv.Atoi(s.CookieLifetime)
	if err != nil {
		minutes = 60
	}

	//should cookie persist?
	if strings.ToLower(s.CookiePersist) == "true" {
		persist = true
	}

	// should cookie be secure
	if strings.ToLower(s.CookieSecure) == "true" {
		secure = true
	}

	//create session
	session := scs.New()

	session.Lifetime = time.Duration(minutes) * time.Minute
	session.Cookie.Persist = persist
	session.Cookie.Secure = secure
	session.Cookie.Name = s.CookieName
	session.Cookie.Domain = s.CookieDomain
	session.Cookie.SameSite = http.SameSiteLaxMode

	// which session store
	switch strings.ToLower(s.SessionType) {
	case "redis":
		session.Store = redisstore.New(s.RedisPool)
	case "mysql", "mariadb":
		session.Store = mysqlstore.New(s.DBPool)
	case "postgres", "postgresql":
		session.Store = postgresstore.New(s.DBPool)
	default:
		//cookies
	}
	return session
}
