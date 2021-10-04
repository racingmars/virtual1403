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
	"net/http"
	"strings"
)

func (app *application) home(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	app.render(w, r, "home.page.tmpl", nil)
}

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

func (app *application) listUsers(w http.ResponseWriter, r *http.Request) {
	users, err := app.db.GetUsers()
	if err != nil {
		http.Error(w, "Internal Server Error", 500)
		return
	}

	app.render(w, r, "users.page.tmpl", users)
}
