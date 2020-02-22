package main

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gorilla/securecookie"
	"github.com/jinzhu/gorm"
	"golang.org/x/crypto/bcrypt"
)

type Role uint32

const (
	Unknown Role = iota
	Admin
	Editor
	User
)

type Account struct {
	ID       uint32
	Name     string `gorm:"unique"`
	Password []byte

	Role Role
}

func parseRole(str string) (Role, error) {
	role, err := strconv.Atoi(str)
	if err != nil || role < 0 || role > 3 {
		return Unknown, fmt.Errorf("Cannot parse '%s' as role", str)
	}
	return Role(role), nil
}

func session(id uint32) *http.Cookie {
	s := securecookie.New(hashKey, blockKey)
	var encoded string
	var err error
	if encoded, err = s.Encode("session", id); err != nil {
		panic(err)
	}
	return &http.Cookie{
		Name:  "session",
		Value: encoded,
	}
}

func (h Handler) login(w http.ResponseWriter, r *http.Request) {
	var acct Account
	acct.Name = r.FormValue("username")
	err := h.db.Where(acct).First(&acct).Error
	if gorm.IsRecordNotFoundError(err) {
		http.Error(w, "user not found", 404)
		return
	} else if err != nil {
		panic(err)
	}
	password := r.FormValue("password")
	if bcrypt.CompareHashAndPassword(acct.Password, []byte(password)) != nil {
		http.Error(w, "wrong password", 401)
		return
	}
	http.SetCookie(w, session(acct.ID))
}

func logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie("session"); err == nil {
		cookie.MaxAge = -1
		http.SetCookie(w, cookie)
	}
}

func (h Handler) register(w http.ResponseWriter, r *http.Request) {
	var acct Account
	var err error
	acct.Name = r.FormValue("username")
	acct.Role, err = parseRole(r.FormValue("role"))
	if err != nil {
		http.Error(w, err.Error(), 415)
		return
	}
	password := r.FormValue("password")
	acct.Password, err = bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		panic(err)
	}
	if err = h.db.Create(&acct).Error; err != nil {
		http.Error(w, "cannot register user "+err.Error(), 500)
	}
}

func (h Handler) auth(next http.HandlerFunc, role Role) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s := securecookie.New(hashKey, blockKey)
		if cookie, err := r.Cookie("session"); err == nil {
			var id uint32
			if err := s.Decode("session", cookie.Value, &id); err == nil {
				var acct Account
				if err = h.db.First(&acct, id).Error; err != nil {
					http.Error(w, "account not found", 401)
					return
				}
				switch {
				case acct.Role == Unknown:
					http.Error(w, "unknown role", 401)
					return
				case acct.Role > role:
					http.Error(w, "permission denied", 401)
					return
				}
				ctx := r.Context()
				ctx = context.WithValue(ctx, KeyAccount, acct)
				r = r.WithContext(ctx)
				next.ServeHTTP(w, r)
				return
			}
			http.Error(w, "session cannot be decode", 401)
			logout(w, r)
			return
		}
		http.Error(w, "session not found", 401)
	}
}
