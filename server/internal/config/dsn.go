package config

import (
	"net"
	"net/url"
)

func (c *Config) DSN() string {
	u := &url.URL{
		Scheme:  "postgres",
		User:    url.UserPassword(c.DBUser, c.DBPass),
		Host:    net.JoinHostPort(c.DBHost, c.DBPort),
		Path:    "/" + c.DBName,
		RawPath: "/" + url.PathEscape(c.DBName),
	}
	q := url.Values{}
	q.Set("sslmode", c.DBSSLMode)
	u.RawQuery = q.Encode()
	return u.String()
}
