package user

import (
	"regexp"

	"github.com/jinzhu/gorm"
	"github.com/nitrous-io/rise-server/dbconn"
)

var emailRe = regexp.MustCompile(`\A[^@\s]+@([^@\s]+\.)+[^@\s]+\z`)

type User struct {
	gorm.Model

	Email        string
	Password     string `sql:"-"`
	Name         string
	Organization string
}

// Returns a struct that can be converted to JSON
func (u *User) AsJSON() interface{} {
	return struct {
		Email        string `json:"email"`
		Name         string `json:"name"`
		Organization string `json:"organization"`
	}{
		u.Email,
		u.Name,
		u.Organization,
	}
}

// Validates User, if there are invalid fields, it returns a map of
// <field, errors> and returns nil if valid
func (u *User) Validate() map[string]string {
	errors := map[string]string{}

	if u.Password == "" {
		errors["password"] = "is required"
	} else if len(u.Password) < 6 {
		errors["password"] = "is too short (min. 6 characters)"
	} else if len(u.Password) > 72 {
		errors["password"] = "is too long (max. 72 characters)"
	}

	if u.Email == "" {
		errors["email"] = "is required"
	} else if len(u.Email) < 5 || !emailRe.MatchString(u.Email) {
		errors["email"] = "is invalid"
	}

	if len(errors) == 0 {
		return nil
	}
	return errors
}

// Inserts the record into the DB, encrypting the Password field
func (u *User) Insert() error {
	db, err := dbconn.DB()
	if err != nil {
		return err
	}

	return db.Table("users").Raw(`INSERT INTO users (
		email,
		encrypted_password
	) VALUES (
		?,
		crypt(?, gen_salt('bf'))
	) RETURNING *;`, u.Email, u.Password).Scan(u).Error
}

// Checks email and password and return user if credentials are valid
func Authenticate(email, password string) (u *User, err error) {
	db, err := dbconn.DB()
	if err != nil {
		return nil, err
	}

	u = &User{}
	if err = db.Where(
		"email = ? AND encrypted_password = crypt(?, encrypted_password)",
		email, password).First(u).Error; err != nil {
		// don't treat record not found as error
		if err == gorm.RecordNotFound {
			return nil, nil
		}
		return nil, err
	}

	return u, err
}
