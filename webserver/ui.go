package main

// Copyright 2021 Matthew R. Wilson <mwilson@mattwilson.org>
//
// This file is part of virtual1403
// <https://github.com/racingmars/virtual1403>.
//
// virtual1403 is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// virtual1403 is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with virtual1403. If not, see <https://www.gnu.org/licenses/>.

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/racingmars/virtual1403/webserver/db"
	"github.com/racingmars/virtual1403/webserver/mailer"
	"github.com/racingmars/virtual1403/webserver/model"
	"golang.org/x/crypto/nacl/auth"
)

// home serves the home page with the login and signup forms. If the user is
// already logged in, we redirect to the user's personal info page.
func (app *application) home(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	// If the user is currently logged in, go to the user page.
	if app.session.GetString(r, "user") != "" {
		http.Redirect(w, r, "user", http.StatusSeeOther)
		return
	}

	responseVars := make(map[string]interface{})
	responseVars["verifySuccess"] = app.session.Get(r, "verifySuccess")
	if responseVars["verifySuccess"] != nil {
		app.session.Remove(r, "verifySuccess")
	}

	// Otherwise, show the front page.
	app.render(w, r, "home.page.tmpl", responseVars)
}

// docsSetup serves the setup documentation page. This is unauthenticated.
func (app *application) docsSetup(w http.ResponseWriter, r *http.Request) {
	responseVars := make(map[string]interface{})
	responseVars["quotaString"] = app.quotaString()
	app.render(w, r, "docs.page.tmpl", responseVars)
}

// docsProfiles serves the setup documentation page. This is unauthenticated.
func (app *application) docsProfiles(w http.ResponseWriter, r *http.Request) {
	responseVars := make(map[string]interface{})
	app.render(w, r, "profiles.page.tmpl", responseVars)
}

// login handles user login requests and if successful sets the session cookie
// user value to the logged in user's email address.
func (app *application) login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	email := strings.TrimSpace(r.PostFormValue("email"))
	pass := r.PostFormValue("password")

	if email == "" || pass == "" {
		app.renderLoginError(w, r, email, "Must provide email and password.")
		return
	}

	u, err := app.db.GetUser(email)
	if err != nil {
		app.renderLoginError(w, r, email, "Invalid login credentials.")
		return
	}

	if !u.CheckPassword(pass) {
		app.renderLoginError(w, r, email, "Invalid login credentials.")
		return
	}

	app.session.Put(r, "user", u.Email)
	http.Redirect(w, r, "user", http.StatusSeeOther)
}

func (app *application) renderLoginError(w http.ResponseWriter,
	r *http.Request, email, message string) {

	app.render(w, r, "home.page.tmpl", map[string]string{
		"loginEmail": email,
		"loginError": message,
	})
}

func (app *application) renderSignupError(w http.ResponseWriter,
	r *http.Request, email, name, message string) {

	app.render(w, r, "home.page.tmpl", map[string]string{
		"signupEmail": email,
		"signupName":  name,
		"signupError": message,
	})
}

// logout destroys the user's session cookie to log them out and sends them
// back to the home page.
func (app *application) logout(w http.ResponseWriter, r *http.Request) {
	app.session.Destroy(r)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// This is the default page for logged-in users
func (app *application) userInfo(w http.ResponseWriter, r *http.Request) {
	// Verify we have a logged in, valid user
	u := app.checkLoggedInUser(r)
	if u == nil {
		// No logged in user
		app.session.Destroy(r)
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	// Get 10 most recent jobs the user printed
	joblog, err := app.db.GetUserJobLog(u.Email, 10)
	if err != nil {
		log.Printf("ERROR: db error getting user joblog for %s: %v",
			u.Email, err)
		// We'll allow the page to render, it'll just have an empty job log
	}
	app.addPDFShareKeys(joblog)

	// Is user currently in violation of quota?
	jobCount, pageCount, quotaErr := app.checkQuota(u.Email)
	if !(quotaErr == nil || quotaErr == errQuotaExceeded) {
		// database error checking quota... we'll set error back to nil for UI
		log.Printf("ERROR: db error in quota check for %s: %v",
			u.Email, quotaErr)
		quotaErr = nil
	}

	quotaMessage := app.quotaString()
	if u.Unlimited {
		quotaMessage = "You are not subject to quotas on this system."
	}

	responseValues := map[string]interface{}{
		"isAdmin":             u.Admin,
		"verified":            u.Verified,
		"name":                u.FullName,
		"email":               u.Email,
		"apiKey":              u.AccessKey,
		"apiEndpoint":         app.serverBaseURL + "/print",
		"pageCount":           u.PageCount,
		"jobCount":            u.JobCount,
		"passwordError":       app.session.Get(r, "passwordError"),
		"passwordSuccess":     app.session.Get(r, "passwordSuccess"),
		"verifySuccess":       app.session.Get(r, "verifySuccess"),
		"verifyResendError":   app.session.Get(r, "verifyResendError"),
		"verifyResendSuccess": app.session.Get(r, "verifyResendSuccess"),
		"joblog":              joblog,
		"quotaMessage":        quotaMessage,
		"quotaViolation":      quotaErr,
		"chargedPages":        pageCount,
		"chargedJobs":         jobCount,
		"pdfRetention":        app.pdfCleanupDays,
		"emailDisabled":       u.DisableEmailDelivery,
		"nuisanceFilter":      !u.AllowNuisanceJobs,
		"serverAdminContact":  app.adminEmail,
	}

	if responseValues["passwordError"] != nil {
		app.session.Remove(r, "passwordError")
	}
	if responseValues["passwordSuccess"] != nil {
		app.session.Remove(r, "passwordSuccess")
	}
	if responseValues["verifySuccess"] != nil {
		app.session.Remove(r, "verifySuccess")
	}
	if responseValues["verifyResendError"] != nil {
		app.session.Remove(r, "verifyResendError")
	}
	if responseValues["verifyResendSuccess"] != nil {
		app.session.Remove(r, "verifyResendSuccess")
	}

	app.render(w, r, "user.page.tmpl", responseValues)
}

// User job log to access more PDFs than the list on the user home page
func (app *application) userJobs(w http.ResponseWriter, r *http.Request) {
	// Verify we have a logged in, valid user
	u := app.checkLoggedInUser(r)
	if u == nil {
		// No logged in user
		app.session.Destroy(r)
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	// Get 100 most recent jobs the user printed
	joblog, err := app.db.GetUserJobLog(u.Email, 100)
	if err != nil {
		log.Printf("ERROR: db error getting user joblog for %s: %v",
			u.Email, err)
		// We'll allow the page to render, it'll just have an empty job log
	}
	app.addPDFShareKeys(joblog)

	responseValues := map[string]interface{}{
		"isAdmin":      u.Admin,
		"joblog":       joblog,
		"pdfRetention": app.pdfCleanupDays,
	}

	app.render(w, r, "userjoblist.page.tmpl", responseValues)
}

// POST hander to regenerate a user's access key
func (app *application) regenkey(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		// Don't accept anything other than a POST
		http.Redirect(w, r, "user", http.StatusSeeOther)
		return
	}

	// Verify we have a logged in, valid user
	u := app.checkLoggedInUser(r)
	if u == nil {
		// No logged in user
		app.session.Destroy(r)
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	// All checks passed... regenerate the key
	u.GenerateAccessKey()
	err := app.db.SaveUser(*u)
	if err != nil {
		log.Printf("ERROR couldn't save user `%s` in DB: %v", u.Email,
			err)
		app.serverError(w, "Sorry, a database error has occurred")
		return
	}

	log.Printf("INFO  %s generated a new access key", u.Email)
	http.Redirect(w, r, "user", http.StatusSeeOther)
}

// adminListUsers provides logged-in administrators with a list of all users in the
// database.
func (app *application) adminListUsers(w http.ResponseWriter, r *http.Request) {
	u := app.checkLoggedInUser(r)
	if u == nil {
		// No logged in user
		app.session.Destroy(r)
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	// Only display this page to administrators
	if !u.Admin {
		w.WriteHeader(http.StatusForbidden)
		io.WriteString(w, "This page is only available to administrators.")
		return
	}

	users, err := app.db.GetUsers()
	if err != nil {
		http.Error(w, "Internal Server Error", 500)
		return
	}

	responseValues := map[string]interface{}{
		"isAdmin": u.Admin,
		"users":   users,
	}

	log.Printf("INFO  %s accessed the users list page", u.Email)

	app.render(w, r, "users.page.tmpl", responseValues)
}

// adminListJobs provides logged-in administrators with a list of the 100 most
// recent jobs.
func (app *application) adminListJobs(w http.ResponseWriter, r *http.Request) {
	u := app.checkLoggedInUser(r)
	if u == nil {
		// No logged in user
		app.session.Destroy(r)
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	// Only display this page to administrators
	if !u.Admin {
		w.WriteHeader(http.StatusForbidden)
		io.WriteString(w, "This page is only available to administrators.")
		return
	}

	jobs, err := app.db.GetJobLog(100)
	if err != nil {
		app.serverError(w, err.Error())
		return
	}

	responseValues := map[string]interface{}{
		"isAdmin": u.Admin,
		"jobs":    jobs,
	}

	log.Printf("INFO  %s accessed the job log page", u.Email)

	app.render(w, r, "jobs.page.tmpl", responseValues)
}

// adminEditUser lets logged-in administrators edit a user.
func (app *application) adminEditUser(w http.ResponseWriter, r *http.Request) {
	u := app.checkLoggedInUser(r)
	if u == nil {
		// No logged in user
		app.session.Destroy(r)
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	// Only display this page to administrators
	if !u.Admin {
		w.WriteHeader(http.StatusForbidden)
		io.WriteString(w, "This page is only available to administrators.")
		return
	}

	// User to edit is email address in query param 'email'
	email := r.URL.Query().Get("email")
	if email == "" {
		http.Error(w, "email query parameter is required",
			http.StatusBadRequest)
		return
	}

	user, err := app.db.GetUser(email)
	if err == db.ErrNotFound {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("ERROR: error getting user: %v", err)
		http.Error(w, "DB error getting user record",
			http.StatusInternalServerError)
		return
	}

	// Get 100 most recent jobs the user printed
	joblog, err := app.db.GetUserJobLog(user.Email, 100)
	if err != nil {
		log.Printf("ERR  db error getting user joblog for %s: %v",
			user.Email, err)
		// We'll allow the page to render, it'll just have an empty job log
	}

	responseValues := map[string]interface{}{
		"isAdmin":              u.Admin, // logged-in user, not target user
		"verified":             user.Verified,
		"name":                 user.FullName,
		"email":                user.Email,
		"admin":                user.Admin,
		"active":               user.Enabled,
		"unlimited":            user.Unlimited,
		"pageCount":            user.PageCount,
		"jobCount":             user.JobCount,
		"joblog":               joblog,
		"signupDate":           user.SignupDate,
		"disableEmailDelivery": user.DisableEmailDelivery,
		"nuisanceFilter":       !u.AllowNuisanceJobs,
	}

	log.Printf("INFO  %s accessed user %s", u.Email, user.Email)

	app.render(w, r, "useredit.page.tmpl", responseValues)
}

// adminEditUserPost lets logged-in administrators submit changes to the user
// record.
func (app *application) adminEditUserPost(w http.ResponseWriter,
	r *http.Request) {

	// Only form POST requests to this handler
	if r.Method != http.MethodPost {
		http.Error(w, "Bad method", http.StatusMethodNotAllowed)
		return
	}

	u := app.checkLoggedInUser(r)
	if u == nil {
		// No logged in user
		app.session.Destroy(r)
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	// Only display this page to administrators
	if !u.Admin {
		w.WriteHeader(http.StatusForbidden)
		io.WriteString(w, "This page is only available to administrators.")
		return
	}

	if err := r.ParseForm(); err != nil {
		log.Printf("ERROR: couldn't parse update user form: %v", err)
		http.Error(w, "couldn't parse update form", http.StatusInternalServerError)
		return
	}

	email := r.Form.Get("email")

	user, err := app.db.GetUser(email)
	if err == db.ErrNotFound {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("ERROR: error getting user: %v", err)
		http.Error(w, "DB error getting user record",
			http.StatusInternalServerError)
		return
	}

	newPassword := r.Form.Get("new-password")
	newName := r.Form.Get("name")
	active := r.Form.Get("active")
	deliverEmail := r.Form.Get("emailDelivery")
	nuisanceFilter := r.Form.Get("nuisanceFilter")
	unlimited := r.Form.Get("unlimited")
	admin := r.Form.Get("admin")

	if newPassword != "" {
		user.SetPassword(newPassword)
	}

	if newName != "" {
		user.FullName = newName
	}

	if active == "yes" {
		user.Enabled = true
	} else {
		user.Enabled = false
	}

	if deliverEmail == "yes" {
		user.DisableEmailDelivery = false
	} else {
		user.DisableEmailDelivery = true
	}

	if nuisanceFilter == "yes" {
		user.AllowNuisanceJobs = false
	} else {
		user.AllowNuisanceJobs = true
	}

	if unlimited == "yes" {
		user.Unlimited = true
	} else {
		user.Unlimited = false
	}

	if admin == "yes" {
		user.Admin = true
	} else {
		user.Admin = false
	}

	if err := app.db.SaveUser(user); err != nil {
		log.Printf("ERROR: saving user %s: %v", user.Email, err)
		http.Error(w, "DB error saving user", http.StatusInternalServerError)
		return
	}

	log.Printf("INFO:  %s updated user %s", u.Email, user.Email)

	http.Redirect(w, r, "users", http.StatusSeeOther)
}

// adminDeleteUser lets logged-in administrators delete a user.
func (app *application) adminDeleteUser(w http.ResponseWriter,
	r *http.Request) {

	// Only POST requests to this handler
	if r.Method != http.MethodPost {
		http.Error(w, "Bad method", http.StatusMethodNotAllowed)
		return
	}

	u := app.checkLoggedInUser(r)
	if u == nil {
		// No logged in user
		app.session.Destroy(r)
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	// Only allow this for administrators
	if !u.Admin {
		w.WriteHeader(http.StatusForbidden)
		io.WriteString(w, "This action is only available to administrators.")
		return
	}

	r.ParseForm()
	email := r.Form.Get("email")
	userToDelete, err := app.db.GetUser(email)
	if err == db.ErrNotFound {
		http.Error(w, "user does not exist", http.StatusNotFound)
		return
	}
	if err != nil {
		app.serverError(w, err.Error())
		return
	}

	if err := app.db.DeleteUser(userToDelete.Email, u.Email); err != nil {
		app.serverError(w, err.Error())
		return
	}
	log.Printf("INFO:  %s deleted user %s", u.Email, userToDelete.Email)

	http.Redirect(w, r, "users", http.StatusSeeOther)
}

// We require a firstname and lastname for signup; we'll assume any two
// space-separated words are okay.
var nameRegexp = regexp.MustCompile(`\S+\s+\S+`)

// signup is the HTTP POST handler for /signup, to create new user accounts.
// If everything is okay, we will create the new user in an unverified state
// and send the new email address the verification email.
func (app *application) signup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	email := r.PostFormValue("email")
	name := r.PostFormValue("name")
	botDetection := r.PostFormValue("website")
	password := r.PostFormValue("password")
	passwordConfirm := r.PostFormValue("password-confirm")

	if botDetection != "" {
		http.Error(w, "Go away, bot", http.StatusForbidden)
		return
	}

	if !mailer.ValidateAddress(strings.ToLower(email)) {
		app.renderSignupError(w, r, email, name,
			"Must provide a valid email address.")
		return
	}

	name = strings.TrimSpace(name)
	if !nameRegexp.MatchString(name) {
		app.renderSignupError(w, r, email, name,
			"Must provide a first and last name.")
		return
	}

	if password == "" || len(password) < 8 {
		app.renderSignupError(w, r, email, name,
			"Must provide a password at least 8 characters long.")
		return
	}

	if password != passwordConfirm {
		app.renderSignupError(w, r, email, name,
			"Passwords do not match.")
		return
	}

	if _, err := app.db.GetUser(email); err != db.ErrNotFound {
		app.renderSignupError(w, r, email, name,
			"That email address already has an account.")
		return
	}

	newuser := model.NewUser(email, password)
	newuser.FullName = name
	newuser.Enabled = true
	newuser.LastVerificationEmail = time.Now()

	if err := app.db.SaveUser(newuser); err != nil {
		log.Printf("ERROR couldn't save new user %s to DB: %v", email, err)
		app.serverError(w, "Unexpected error saving new user to database.")
		return
	}

	app.session.Put(r, "user", newuser.Email)

	if err := mailer.SendVerificationCode(app.mailconfig, newuser.Email,
		app.serverBaseURL+"/verify?token="+
			url.QueryEscape(newuser.AccessKey)); err != nil {
		log.Printf("ERROR couldn't send verification email for %s: %v",
			newuser.Email, err)
	}

	http.Redirect(w, r, "user", http.StatusSeeOther)
}

// resendVerification is the HTTP hander for POSTs to /resent for users that
// want to re-send the verification email. We will only allow one send per
// hour so that we can't be used for someone to sign up with someone else's
// email address then blast it with spam.
func (app *application) resendVerification(w http.ResponseWriter,
	r *http.Request) {

	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	// Verify we have a logged in, valid user
	u := app.checkLoggedInUser(r)
	if u == nil {
		// No logged in user
		app.session.Destroy(r)
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	if u.Verified {
		// User is already verified. Just send them back to user page.
		log.Printf(
			"INFO  %s tried to send verification email but is already verified",
			u.Email)
		http.Redirect(w, r, "/user", http.StatusSeeOther)
		return
	}

	if time.Since(u.LastVerificationEmail) < 1*time.Hour {
		// Too soon... make them wait an hour between verifications.
		app.session.Put(r, "verifyResendError", "We have sent a "+
			"verification email within the last hour. Please wait for the "+
			"email to arrive, or request the email again after an hour "+
			"has passed.")
		log.Printf(
			"INFO  %s tried to request another verification email too quickly",
			u.Email)
		http.Redirect(w, r, "/user", http.StatusSeeOther)
		return
	}

	// Update the user's last verification send time
	u.LastVerificationEmail = time.Now()
	if err := app.db.SaveUser(*u); err != nil {
		log.Printf("ERROR couldn't save updated user %s to DB: %v",
			u.Email, err)
		app.serverError(w, "Unexpected error saving user update to database.")
		return
	}

	if err := mailer.SendVerificationCode(app.mailconfig, u.Email,
		app.serverBaseURL+"/verify?token="+
			url.QueryEscape(u.AccessKey)); err != nil {
		log.Printf("ERROR couldn't send verification email for %s: %v",
			u.Email, err)
		app.session.Put(r, "verifyResendError",
			"Error sending verification email.")
	} else {
		app.session.Put(r, "verifyResendSuccess",
			"Verification email sent.")
	}

	http.Redirect(w, r, "user", http.StatusSeeOther)

}

// changePassword is the HTTP handler for POSTS to /changepassword for users
// to change their password. We verify that a valid user is logged in, then
// change the password if the old password checks out.
func (app *application) changePassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	// Verify we have a logged in, valid user
	u := app.checkLoggedInUser(r)
	if u == nil {
		// No logged in user
		app.session.Destroy(r)
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	if !u.CheckPassword(r.PostFormValue("password")) {
		// Users existing password does not match
		app.session.Put(r, "passwordError",
			"Your current password was incorrect.")
		log.Printf("INFO  %s unsuccessfully attempted password change", u.Email)
		http.Redirect(w, r, "user", http.StatusSeeOther)
		return
	}

	newPassword := r.PostFormValue("new-password")
	newPassword2 := r.PostFormValue("new-password2")

	if len(newPassword) < 8 {
		app.session.Put(r, "passwordError",
			"Your new password must be 8 or more characters long.")
		log.Printf("INFO  %s unsuccessfully attempted password change", u.Email)
		http.Redirect(w, r, "user", http.StatusSeeOther)
		return
	}

	if newPassword != newPassword2 {
		app.session.Put(r, "passwordError",
			"New passwords do not match.")
		log.Printf("INFO  %s unsuccessfully attempted password change", u.Email)
		http.Redirect(w, r, "user", http.StatusSeeOther)
		return
	}

	u.SetPassword(newPassword)
	if err := app.db.SaveUser(*u); err != nil {
		app.serverError(w, "Sorry, a database error has occurred")
		return
	}

	app.session.Put(r, "passwordSuccess",
		"Your password was successfully changed.")
	log.Printf("INFO  %s successfully changed their password", u.Email)
	http.Redirect(w, r, "user", http.StatusSeeOther)
}

// verifyUser is the HTTP hander for /verify; we expect /verify?token=... to
// verify a user after sending them the email verification link. If the
// verification token belongs to an unverified account, we will set the
// account to verified and generate a new token (which will be used as their
// print API access token going forward).
func (app *application) verifyUser(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		http.NotFound(w, r)
		return
	}

	u, err := app.db.GetUserForAccessKey(token)
	if err == db.ErrNotFound {
		w.WriteHeader(http.StatusNotFound)
		io.WriteString(w, "That token was not found or has already been "+
			"used to verify the user.")
		return
	}
	if err != nil {
		app.serverError(w, "Unexpected error looking up token")
		return
	}

	if u.Verified {
		w.WriteHeader(http.StatusConflict)
		io.WriteString(w, "That user has already been verified.")
		return
	}

	u.Verified = true
	u.GenerateAccessKey()

	if err := app.db.SaveUser(u); err != nil {
		app.serverError(w, "Error saving user record after verification")
		return
	}

	app.session.Put(r, "verifySuccess", "Email address successfully verified.")
	log.Printf("INFO  %s verified their account", u.Email)
	http.Redirect(w, r, "user", http.StatusSeeOther)
}

// checkLoggedInUser checks if a user is logged in to the session in the HTTP
// request r. If there is a valid session cookie with a username, we check
// that the user still exists and isn't disabled. If everything is good, we
// return a pointer to the logged-in user. If the user isn't logged in or
// there is a problem, we return nil.
func (app *application) checkLoggedInUser(r *http.Request) *model.User {
	username := app.session.GetString(r, "user")
	if username == "" {
		return nil
	}

	user, err := app.db.GetUser(username)
	if err == db.ErrNotFound {
		log.Printf(
			"INFO  user `%s` has a session cookie but the account no "+
				"longer exists",
			username)
		return nil
	}
	if err != nil {
		log.Printf("ERROR couldn't look up user `%s` in DB: %v", username,
			err)
		return nil
	}

	if !user.Enabled {
		log.Printf(
			"INFO  user `%s` has a session cookie but the account is disabled",
			username)
		return nil
	}

	// At this point, we have a valid, active logged-in user.
	return &user
}

func (app *application) pdf(w http.ResponseWriter, r *http.Request) {
	keyStr := r.URL.Query().Get("sharekey")
	if keyStr == "" {
		http.Error(w, "keyStr query parameter must be present",
			http.StatusBadRequest)
		return
	}

	// Can we hex decode, and is the result the length of our message (a
	// uint64) plus signature?
	keyRaw, err := hex.DecodeString(keyStr)
	if err != nil || len(keyRaw) != 64/8+auth.Size {
		http.Error(w, "keyStr is invalid", http.StatusBadRequest)
		return
	}

	msg := keyRaw[0 : 64/8] // uint64
	sig := keyRaw[64/8:]
	if !auth.Verify(sig, msg, app.shareKey) {
		// Signature verification failed; this is not a genuine PDF link.
		// We will treat all failures as 404 not found.
		http.Error(w, "PDF for job no longer available", http.StatusNotFound)
		return
	}

	id, err := bytesToUint64BE(msg)
	if err != nil {
		// Invalid message...which shouldn't be possible since we already
		// verified the signature and our code should only have created
		// correct messages in the first place.
		log.Printf("ERROR: valid signature on invalid message: %s", keyStr)
		http.Error(w, "PDF for job no longer available", http.StatusNotFound)
		return
	}

	job, err := app.db.GetJob(id)
	if err == db.ErrNotFound {
		http.Error(w, "PDF for job no longer available", http.StatusNotFound)
		return
	}
	if err != nil {
		app.serverError(w, "db error getting job for PDF retrieval: "+
			err.Error())
		return
	}

	pdf, err := app.db.GetPDF(id)
	if err == db.ErrNotFound {
		http.Error(w, "PDF for job no longer available", http.StatusNotFound)
		return
	}
	if err != nil {
		app.serverError(w, "db error retrieving PDF: "+err.Error())
		return
	}

	log.Printf("INFO:  Retrieved PDF for job %d", id)

	jobtag := job.JobInfo
	if jobtag != "" {
		jobtag = jobtag + "-"
	}
	jobname := fmt.Sprintf("%s%s", jobtag,
		job.Time.UTC().Format("2006-01-02T150405Z"))

	w.Header().Add("Content-Type", "application/pdf")
	w.Header().Add("Content-Disposition",
		fmt.Sprintf("inline; filename=\"virtual1403_%s.pdf\"", jobname))
	w.Header().Add("Content-Length", strconv.Itoa(len(pdf)))
	w.WriteHeader(http.StatusOK)
	w.Write(pdf)
}

func (app *application) changeDelivery(w http.ResponseWriter, r *http.Request) {
	// Verify we have a logged in, valid user
	u := app.checkLoggedInUser(r)
	if u == nil {
		// No logged in user
		app.session.Destroy(r)
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Only POST is allowed", http.StatusMethodNotAllowed)
		return
	}

	action := r.URL.Query().Get("action")
	switch action {
	case "disable":
		u.DisableEmailDelivery = true
	case "enable":
		u.DisableEmailDelivery = false
	default:
		http.Error(w, "Action must be disable or enable",
			http.StatusBadRequest)
		return
	}

	if err := app.db.SaveUser(*u); err != nil {
		app.serverError(w, err.Error())
		return
	}

	log.Printf("INFO:  User %s changed email delivery preference: %s", u.Email,
		action)
	http.Redirect(w, r, "user", http.StatusSeeOther)
}

func (app *application) changeNuisance(w http.ResponseWriter, r *http.Request) {
	// Verify we have a logged in, valid user
	u := app.checkLoggedInUser(r)
	if u == nil {
		// No logged in user
		app.session.Destroy(r)
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Only POST is allowed", http.StatusMethodNotAllowed)
		return
	}

	action := r.URL.Query().Get("action")
	switch action {
	case "disable":
		u.AllowNuisanceJobs = false
	case "enable":
		u.AllowNuisanceJobs = true
	default:
		http.Error(w, "Action must be disable or enable",
			http.StatusBadRequest)
		return
	}

	if err := app.db.SaveUser(*u); err != nil {
		app.serverError(w, err.Error())
		return
	}

	log.Printf("INFO:  User %s changed nuisance job preference: %s", u.Email,
		action)
	http.Redirect(w, r, "user", http.StatusSeeOther)
}

func (app *application) addPDFShareKeys(jobs []model.JobLogEntry) {
	for i := range jobs {
		if !jobs[i].HasPDF {
			continue
		}

		// Encode the ID and sign it
		logID := uint64ToBytesBE(jobs[i].ID)
		sig := auth.Sum(logID, app.shareKey)
		logID = append(logID, sig[:]...)
		jobs[i].ShareKey = hex.EncodeToString(logID)
	}
}

func uint64ToBytesBE(in uint64) []byte {
	var buf bytes.Buffer
	binary.Write(&buf, binary.BigEndian, &in)
	return buf.Bytes()
}

func bytesToUint64BE(in []byte) (uint64, error) {
	inrdr := bytes.NewReader(in)
	var out uint64
	err := binary.Read(inrdr, binary.BigEndian, &out)
	return out, err
}
