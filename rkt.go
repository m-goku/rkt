package rkt

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/CloudyKit/jet/v6"
	"github.com/alexedwards/scs/v2"
	"github.com/dgraph-io/badger/v3"
	"github.com/go-chi/chi/v5"
	"github.com/gomodule/redigo/redis"
	"github.com/m-goku/rkt/cache"
	"github.com/m-goku/rkt/mailer"
	"github.com/m-goku/rkt/render"
	"github.com/m-goku/rkt/sessions"
	"github.com/robfig/cron/v3"
)

const version = "1.0.0"

var myRedisCache *cache.RedisCache
var myBadgerCache *cache.BadgerCache

var redisPool *redis.Pool
var badgerConn *badger.DB

/*
Create the Rocket struct
AppName - the name of the app
Debug - mode for development
Version - current app version
*/
type RKT struct {
	AppName       string
	Debug         bool
	Version       string
	ErrorLog      *log.Logger
	InfoLog       *log.Logger
	RootPath      string
	Routes        *chi.Mux
	Render        *render.Render
	Session       *scs.SessionManager
	DB            Database
	JetViews      *jet.Set
	config        config
	EncryptionKey string
	Cache         cache.Cache
	Scheduler     *cron.Cron
	Mail          mailer.Mail
	Server        Server
}

type config struct {
	port        string
	renderer    string
	cookie      cookieConfig
	sessionType string
	database    databaseConfig
	redis       redisConfig
}

type Server struct {
	ServerName string
	Port       string
	Secure     bool
	URL        string
}

/*
New method is to create an instance of the Rocket struct.
It has the Init method that that takes the path and create -
the neccessary folders for the project
pathConfig is a variable for the instance of the initPath struct
*/
func (r *RKT) New(rootPath string) error {
	pathConfig := initPaths{
		rootPath: rootPath,
		folderNames: []string{
			"handlers",
			"migrations",
			"views",
			"mail",
			"data",
			"public",
			"temp",
			"logs",
			"middleware",
		},
	}

	//initialize path and create folders
	err := r.Init(pathConfig)
	if err != nil {
		return err
	}

	
    if os.Getenv("RENDER") == "" {
        if err := godotenv.Load(".env"); err != nil {
            log.Println("⚠️ No .env file found locally, skipping...")
        } else {
            log.Println("✅ Loaded .env file for local development")
        }
    } else {
        log.Println("✅ Running on Render - using Render environment variables")
    }


	//create loggers
	infoLog, errorLog := r.startLoggers()

	// connect to database
	if os.Getenv("DATABASE_TYPE") != "" {
		if os.Getenv("DATABASE_TYPE") == "mongodb" {
			mongoClient, err := r.OpenMongoDB(r.BuildDSN())
			if err != nil {
				errorLog.Println(err)
				os.Exit(1)
			}
			//defer mongoClient.Disconnect(context.Background())

			r.DB = Database{
				DataType: os.Getenv("DATABASE_TYPE"),
				Conn:     mongoClient,
			}

			infoLog.Println("Connected to MongoDB!")

		} else if os.Getenv("DATABASE_TYPE") == "postgresql" || os.Getenv("DATABASE_TYPE") == "postgres" {
			db, err := r.OpenPostgresDB(os.Getenv("DATABASE_TYPE"), r.BuildDSN())
			if err != nil {
				errorLog.Println(err)
				os.Exit(1)
			}
			r.DB = Database{
				DataType: os.Getenv("DATABASE_TYPE"),
				Pool:     db,
			}
			infoLog.Println("Connected to Postgresql!")
		}
	}

	scheduler := cron.New()
	r.Scheduler = scheduler

	if os.Getenv("CACHE") == "redis" || os.Getenv("SESSION_TYPE") == "redis" {
		myRedisCache = r.createClientRedisCache()
		r.Cache = myRedisCache
		redisPool = myRedisCache.Conn
	}

	if os.Getenv("CACHE") == "badger" {
		myBadgerCache = r.createClientBadgerCache()
		r.Cache = myBadgerCache
		badgerConn = myBadgerCache.Conn

		_, err = r.Scheduler.AddFunc("@daily", func() {
			_ = myBadgerCache.Conn.RunValueLogGC(0.7)
		})
		if err != nil {
			return err
		}
	}

	r.InfoLog = infoLog
	r.ErrorLog = errorLog
	r.Debug, _ = strconv.ParseBool(os.Getenv("DEBUG"))
	r.Version = version
	r.RootPath = rootPath
	r.Mail = r.createMailer()
	//set routes
	r.Routes = r.routes().(*chi.Mux)

	//set configurations
	r.config = config{
		port:     os.Getenv("PORT"),
		renderer: os.Getenv("RENDERER"),
		cookie: cookieConfig{
			name:     os.Getenv("COOKIE_NAME"),
			lifetime: os.Getenv("COOKIE_LIFETIME"),
			persists: os.Getenv("COOKIE_PERSISTS"),
			secure:   os.Getenv("COOKIE_SECURE"),
			domain:   os.Getenv("COOKIE_DOMAIN"),
		},
		sessionType: os.Getenv("SESSION_TYPE"),
		database: databaseConfig{
			database: os.Getenv("DATABASE_TYPE"),
			dsn:      r.BuildDSN(),
		},
		redis: redisConfig{
			host:     os.Getenv("REDIS_HOST"),
			password: os.Getenv("REDIS_PASSWORD"),
			prefix:   os.Getenv("REDIS_PREFIX"),
		},
	}

	secure := true
	if strings.ToLower(os.Getenv("SECURE")) == "false" {
		secure = false
	}

	r.Server = Server{
		ServerName: os.Getenv("SERVER_NAME"),
		Port:       os.Getenv("PORT"),
		Secure:     secure,
		URL:        os.Getenv("APP_URL"),
	}

	// create session
	session := sessions.Session{
		CookieLifetime: r.config.cookie.lifetime,
		CookiePersist:  r.config.cookie.persists,
		CookieName:     r.config.cookie.name,
		CookieDomain:   r.config.cookie.domain,
		CookieSecure:   r.config.cookie.secure,
		SessionType:    r.config.sessionType,
		DBPool:         r.DB.Pool,
	}

	switch r.config.sessionType {
	case "redis":
		session.RedisPool = myRedisCache.Conn
	case "mysql", "postgres", "mariadb", "postgresql":
		session.DBPool = r.DB.Pool
	}

	r.Session = session.InitSession()
	r.EncryptionKey = os.Getenv("KEY")

	if r.Debug {
		var views = jet.NewSet(
			jet.NewOSFileSystemLoader(fmt.Sprintf("%s/views", rootPath)),
			jet.InDevelopmentMode(),
		)
		r.JetViews = views
	} else {
		var views = jet.NewSet(
			jet.NewOSFileSystemLoader(fmt.Sprintf("%s/views", rootPath)),
		)
		r.JetViews = views
	}

	r.createRenderer()

	return nil
}

/*
Init method that that takes the initPath struct and create -
the neccessary folders for the project into the path provided
CreateDirIfNotExist creates the folders into the path provided.
*/
func (r *RKT) Init(p initPaths) error {
	rootPath := p.rootPath
	for _, folder := range p.folderNames {
		err := r.CreateDirIfNotExist(rootPath + "/" + folder)
		if err != nil {
			return err
		}
	}
	return nil
}

// Listen and serve starts the web server
func (r *RKT) ListenAndServe() {
	server := &http.Server{
		Addr:         ":" + os.Getenv("PORT"),
		ErrorLog:     r.ErrorLog,
		Handler:      r.Routes,
		IdleTimeout:  30 * time.Second,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 600 * time.Second,
	}

	//close database connections
	if r.DB.Conn != nil {
		defer r.DB.Conn.Disconnect(context.Background())
	}

	if r.DB.Pool != nil {
		defer r.DB.Pool.Close()
	}

	if redisPool != nil {
		defer redisPool.Close()
	}

	if badgerConn != nil {
		defer badgerConn.Close()
	}

	r.InfoLog.Println("Listening on Port" + os.Getenv("PORT"))
	err := server.ListenAndServe()
	r.ErrorLog.Fatal(err)
}

func (r *RKT) checkDotEnv(path string) error {
	err := r.CreateFileIfNotExists(path)
	if err != nil {
		return err
	}
	return nil
}

func (r *RKT) startLoggers() (*log.Logger, *log.Logger) {
	var infoLog *log.Logger
	var errorLog *log.Logger

	infoLog = log.New(os.Stdout, "INFO\t", log.Ldate|log.Ltime)
	errorLog = log.New(os.Stdout, "ERROR\t", log.Ldate|log.Ltime|log.Lshortfile)
	return infoLog, errorLog
}

func (r *RKT) createRenderer() {
	myRenderer := &render.Render{
		Renderer: r.config.renderer,
		RootPath: r.RootPath,
		Port:     r.config.port,
		JetViews: r.JetViews,
		Session:  r.Session,
	}
	r.Render = myRenderer
}

func (c *RKT) createMailer() mailer.Mail {
	//port, _ := strconv.Atoi(os.Getenv("SMTP_PORT"))
	m := mailer.Mail{
		Templates:   c.RootPath + "/mail",
		FromName:    os.Getenv("FROM_NAME"),
		FromAddress: os.Getenv("FROM_ADDRESS"),
		PublicAPI:   os.Getenv("PUBLIC_API"),
		PrivateAPI:  os.Getenv("PRIVATE_API"),
		// Domain:      os.Getenv("MAIL_DOMAIN"),
		//Host:        os.Getenv("SMTP_HOST"),
		//Port:        port,
		// Username:    os.Getenv("SMTP_USERNAME"),
		// Password:    os.Getenv("SMTP_PASSWORD"),
		// Encryption:  os.Getenv("SMTP_ENCRYPTION"),
		// API:         os.Getenv("MAILER_API"),
		// APIKey:      os.Getenv("MAILER_KEY"),
		// APIUrl:      os.Getenv("MAILER_URL"),
	}
	return m
}

// BuildDSN builds the datasource name for our database, and returns it as a string
func (c *RKT) BuildDSN() string {
	var dsn string

	switch os.Getenv("DATABASE_TYPE") {
	case "postgres", "postgresql", "mongodb", "mongo":
		dsn = os.Getenv("DATABASE_CONN_STR")
		// dsn = fmt.Sprintf("host=%s port=%s user=%s dbname=%s sslmode=%s timezone=UTC connect_timeout=5",
		// 	os.Getenv("DATABASE_HOST"),
		// 	os.Getenv("DATABASE_PORT"),
		// 	os.Getenv("DATABASE_USER"),
		// 	os.Getenv("DATABASE_NAME"),
		// 	os.Getenv("DATABASE_SSL_MODE"))

		// we check to see if a database passsword has been supplied, since including "password=" with nothing
		// after it sometimes causes postgres to fail to allow a connection.
		// if os.Getenv("DATABASE_PASS") != "" {
		// 	dsn = fmt.Sprintf("%s password=%s", dsn, os.Getenv("DATABASE_PASS"))
		// }

	default:

	}

	return dsn
}

func (c *RKT) createRedisPool() *redis.Pool {
	return &redis.Pool{
		MaxIdle:     50,
		MaxActive:   10000,
		IdleTimeout: 240 * time.Second,
		Dial: func() (redis.Conn, error) {
			return redis.Dial("tcp",
				c.config.redis.host,
				redis.DialPassword(c.config.redis.password))
		},

		TestOnBorrow: func(conn redis.Conn, t time.Time) error {
			_, err := conn.Do("PING")
			return err
		},
	}
}

func (c *RKT) createClientRedisCache() *cache.RedisCache {
	cacheClient := cache.RedisCache{
		Conn:   c.createRedisPool(),
		Prefix: c.config.redis.prefix,
	}
	return &cacheClient
}

func (c *RKT) createClientBadgerCache() *cache.BadgerCache {
	cacheClient := cache.BadgerCache{
		Conn: c.createBadgerConn(),
	}
	return &cacheClient
}

func (c *RKT) createBadgerConn() *badger.DB {
	db, err := badger.Open(badger.DefaultOptions(c.RootPath + "/tmp/badger"))
	if err != nil {
		return nil
	}
	return db
}
