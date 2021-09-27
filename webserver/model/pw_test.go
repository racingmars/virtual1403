package model

import (
	"testing"
)

func TestUserPasswords(t *testing.T) {
	const password = "test1234"

	var u User

	// First make sure raw password value doesn't work
	u.PasswordHash = password
	if u.CheckPassword(password) {
		t.Error("Unhashed password was accepted.")
	}

	// Now set a password correctly
	u.SetPassword(password)
	if u.PasswordHash == password {
		t.Errorf("The plaintext password was set in PasswordHash")
	}

	// And verify it
	if u.CheckPassword("") {
		t.Error("Blank password allowed to log in")
	}
	if u.CheckPassword("not-the-password") {
		t.Error("Incorrect password allowed to log in")
	}
	if !u.CheckPassword(password) {
		t.Error("Correct password not allowed to log ing")
	}

	u = User{}
	if u.CheckPassword("") {
		t.Error("Blank password allowed when hash is blank")
	}
}

func TestUserAccessKey(t *testing.T) {
	var u User

	u.GenerateAccessKey()
	if u.AccessKey == "" {
		t.Error("User has a blank access key")
	}

	t.Log(u.AccessKey)

	oldkey := u.AccessKey
	u.GenerateAccessKey()
	if u.AccessKey == oldkey {
		t.Error("Generating new access key didn't change the value")
	}
}
