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
	"errors"
	"fmt"
	"strconv"
	"time"
)

var errQuotaExceeded = errors.New("quota exceeded")

// checkQuota will determine if users are currently exceeding their quota of
// printing during the quota period. Will return nil if the user is allowed to
// print more, or errQuotaExceeded if not. Other errors during database lookup
// will be returned as-is.
func (app *application) checkQuota(email string) (jobs, pages int, err error) {
	// If no quotas enabled, return immediately.
	if app.quotaJobs <= 0 && app.quotaPages <= 0 {
		return 0, 0, nil
	}

	user, err := app.db.GetUser(email)
	if err != nil {
		return 0, 0, err
	}

	// Quotas are not enforced against unlimited users
	if user.Unlimited {
		return 0, 0, nil
	}

	// If there is a jobs quota, we only need to retrieve that number of
	// previous jobs. If there is no jobs quota, only a pages quota, we'll
	// check the previous 100 jobs. This does allow for a potential abuse path
	// -- if this is a concern, always set both a jobs and pages quota.
	tmpSize := app.quotaJobs
	if tmpSize <= 0 {
		tmpSize = 100
	}

	log, err := app.db.GetUserJobLog(email, tmpSize)
	if err != nil {
		return 0, 0, err
	}

	periodStart := time.Now().Add(-app.quotaPeriod)

	// the job log is in descending order: the most recent jobs are at the
	// beginning, going back to older jobs as we go through the array. We will
	// proceed until we either exceed a quota or find a job outside the
	// current quota period (in which case the user has not exceeded the
	// quota).
	for i := range log {
		if log[i].Time.Before(periodStart) {
			// quota not exceeded; we're out of jobs that could contribute to
			// the counts in this quota period.
			break
		}
		jobs++
		pages += log[i].Pages

		// Is there a jobs quota and has the user exceeded it?
		if app.quotaJobs > 0 && jobs >= app.quotaJobs {
			return jobs, pages, errQuotaExceeded
		}

		// Is there a pages quota and has the user exceeded it?
		if app.quotaPages > 0 && pages >= app.quotaPages {
			return jobs, pages, errQuotaExceeded
		}
	}

	// Quota not exceeded
	return jobs, pages, nil
}

// quotaString will provide a user-friendly description of the allowed quota.
func (app *application) quotaString() string {
	jobsQuota := "unlimited"
	pagesQuota := "unlimited"

	if app.quotaJobs > 0 {
		jobsQuota = strconv.Itoa(app.quotaJobs)
	}
	if app.quotaPages > 0 {
		pagesQuota = strconv.Itoa(app.quotaPages)
	}

	period := app.quotaPeriod / time.Hour
	periodUnit := "hours"
	if period == 1 {
		periodUnit = "hour"
	}

	deletePeriod := ""
	if app.inactiveMonthsCleanup > 0 {
		deletePeriod = fmt.Sprintf(" Accounts without a print job in the "+
			"previous %d months will be deleted.", app.inactiveMonthsCleanup)
	}

	return fmt.Sprintf(
		"Users are allowed %s jobs and %s pages during the previous %d %s.%s",
		jobsQuota, pagesQuota, period, periodUnit, deletePeriod)
}
