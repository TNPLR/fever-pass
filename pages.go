package main

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/jinzhu/gorm"
)

const (
	PageLimit = 100
)

func (h Handler) index(w http.ResponseWriter, r *http.Request) {
	acct, ok := session(r)
	if ok {
		records, err := h.lastRecords(acct, 3)
		if err == nil {
			h.HTML(w, r, "index.htm", records)
			return
		}
	}
	h.HTML(w, r, "index.htm", nil)
}

// get the last record today of the account
func (h Handler) lastRecords(account Account, n int) (record []Record, err error) {
	err = h.db.Set("gorm:auto_preload", true).Where("created_at > ?", today()).Order("id desc").Limit(n).Find(&record, "account_id = ?", account.ID).Error
	if err != nil {
		panic(err)
	}
	return
}

func (h Handler) newRecordPage(w http.ResponseWriter, r *http.Request) {
	var records []Record
	acct, ok := r.Context().Value(KeyAccount).(Account)
	if ok {
		err := h.listRecord(acct).Where("recorder_id = ?", acct.ID).Limit(100).Find(&records).Error
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
	} else {
		http.Error(w, "cannot read account from session", 500)
		return
	}

	class := ""
	if acct.Authority.Record == Group {
		class = acct.Class.Name
	}

	page := struct {
		Class     string
		PageLimit int
		Records   []Record
	}{class, PageLimit, records}
	h.HTML(w, r, "new.htm", page)
}

func (h Handler) listRecordsPage(w http.ResponseWriter, r *http.Request) {
	records := make([]Record, 0, PageLimit)
	p, err := strconv.Atoi(r.FormValue("page"))
	if err != nil {
		p = 1
	}
	// acct must have value
	title := ""
	acct, _ := session(r)
	tx := h.listRecord(acct)
	tx = joinClasses(tx)
	if id := r.FormValue("account_id"); id != "" {
		account, err := h.getAccount(id)
		if err == nil {
			title += account.Name + " "
			tx = tx.Where("account_id = ?", id)
		}
	}
	if class := r.FormValue("class"); class != "" {
		tx = whereClass(tx, class)
		title += class + "班 "
	}
	if number := r.FormValue("number"); number != "" {
		tx = whereNumber(tx, number)
		title += number + "號 "
	}

	date, err := time.ParseInLocation("2006-01-02", r.FormValue("date"), time.Local)
	if err == nil {
		tx = whereDate(tx, date)
		title += date.Format("01/02 ")
	}

	err = tx.Offset((p - 1) * PageLimit).Limit(PageLimit).Find(&records).Error
	if err != nil {
		panic(err)
	}

	page := make(map[string]interface{})
	page["Page"] = p
	page["Title"] = title
	page["PageLimit"] = PageLimit
	page["Records"] = records
	page["Count"] = pageCount(tx)
	h.HTML(w, r, "list.htm", page)
}

func pageCount(tx *gorm.DB) int {
	var count int
	tx.Count(&count)
	if count%PageLimit == 0 {
		count /= PageLimit
	} else {
		count /= PageLimit
		count++
	}
	return count
}

func (h Handler) listAccountsPage(w http.ResponseWriter, r *http.Request) {
	acct := r.Context().Value(KeyAccount).(Account)
	var accounts []Account

	p, err := strconv.Atoi(r.FormValue("page"))
	if err != nil {
		p = 1
	}
	title := ""

	tx := h.listAccounts(acct)
	if class := r.FormValue("class"); class != "" {
		tx = whereClass(tx, class)
		title += class + "班 "
	}
	if number := r.FormValue("number"); number != "" {
		tx = whereNumber(tx, number)
		title += number + "號 "
	}

	err = tx.Offset(100 * (p - 1)).Limit(100).Find(&accounts).Error
	if err != nil {
		panic(err)
	}

	page := make(map[string]interface{})
	page["Page"] = p
	page["Title"] = title
	page["Accounts"] = accounts
	page["Count"] = pageCount(tx)
	h.HTML(w, r, "account_list.htm", page)
}

func (h Handler) resetPage(w http.ResponseWriter, r *http.Request) {
	acct, _ := session(r)
	account, err := h.getAccount(r.FormValue("account_id"))
	if err == AccountNotFound {
		account = acct
	}

	msg, ok := r.Context().Value(KeyMessage).(string)
	if !ok {
		msg = ""
	}

	if !accountPermission(acct, account) {
		h.errorPage(w, r, "權限不足", "您沒有權限變更"+account.Name+"的密碼")
		return
	}

	page := struct {
		Account
		Message string
	}{
		Account: account,
		Message: msg,
	}

	h.HTML(w, r, "reset.htm", page)
}

func addMessage(r *http.Request, msg string) *http.Request {
	ctx := r.Context()
	ctx = context.WithValue(ctx, KeyMessage, msg)
	return r.WithContext(ctx)
}

func (h Handler) registerPage(w http.ResponseWriter, r *http.Request) {
	page := make(map[string]interface{})
	page["authorities"] = Authorities
	h.HTML(w, r, "register.htm", page)
}

func (h Handler) profile(w http.ResponseWriter, r *http.Request) {
	acct, _ := session(r)
	account, err := h.getAccount(mux.Vars(r)["id"])
	if err == AccountNotFound {
		h.errorPage(w, r, "此帳號不存在", "")
		return
	}

	if !accountPermission(acct, account) && !recordPermission(acct, account) {
		h.errorPage(w, r, "權限不足", "你沒有權限查看此頁面")
		return
	}

	h.HTML(w, r, "profile.htm", account)
}
