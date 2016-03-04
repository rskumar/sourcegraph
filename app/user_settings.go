package app

import (
	"crypto/md5"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/sourcegraph/mux"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"src.sourcegraph.com/sourcegraph/app/internal/authutil"
	"src.sourcegraph.com/sourcegraph/app/internal/tmpl"
	"src.sourcegraph.com/sourcegraph/app/router"
	"src.sourcegraph.com/sourcegraph/errcode"
	"src.sourcegraph.com/sourcegraph/go-sourcegraph/sourcegraph"
	"src.sourcegraph.com/sourcegraph/util/handlerutil"
	"src.sourcegraph.com/sourcegraph/util/httputil/httpctx"
	"src.sourcegraph.com/sourcegraph/util/router_util"
)

type userSettingsCommonData struct {
	User        *sourcegraph.User
	OrgsAndSelf []*sourcegraph.User
}

var errUserSettingsCommonWroteResponse = errors.New("userSettingsCommon already wrote an HTTP response")

// userSettingsCommon should be called at the beginning of each HTTP
// handler that generates or saves settings data. It checks auth and
// fetches common data.
//
// If this function returns the error
// errUserSettingsCommonWroteResponse, callers should return nil and
// stop handling the HTTP request. That means that this function
// already sent an HTTP response (such as a redirect). For example:
//
// 	userSpec, cd, err := userSettingsCommon(w, r)
// 	if err == errUserSettingsCommonWroteResponse {
// 		return nil
// 	} else if err != nil {
// 		return err
// 	}
func userSettingsCommon(w http.ResponseWriter, r *http.Request) (sourcegraph.UserSpec, *userSettingsCommonData, error) {
	ctx, cl := handlerutil.Client(r)

	currentUser := handlerutil.UserFromRequest(r)
	if currentUser == nil {
		if err := authutil.RedirectToLogIn(w, r); err != nil {
			return sourcegraph.UserSpec{}, nil, err
		}
		return sourcegraph.UserSpec{}, nil, errUserSettingsCommonWroteResponse // tell caller to not continue handling this req
	}

	if err := userSettingsMeRedirect(w, r, currentUser); err == errUserSettingsCommonWroteResponse {
		return sourcegraph.UserSpec{}, nil, err
	}

	p, userSpec, err := getUser(ctx, r)
	if err != nil {
		return sourcegraph.UserSpec{}, nil, err
	}

	if currentUser.UID != userSpec.UID {
		return sourcegraph.UserSpec{}, nil, &errcode.HTTPErr{Status: http.StatusUnauthorized, Err: fmt.Errorf("must be logged in as the requested user")}
	}

	// The settings panel should have sections for the user AND for
	// each of the orgs that the user can admin. This list is
	// orgsAndSelf.
	orgs, err := cl.Orgs.List(ctx, &sourcegraph.OrgsListOp{Member: sourcegraph.UserSpec{UID: int32(currentUser.UID)}, ListOptions: sourcegraph.ListOptions{PerPage: 100}})
	if errcode.GRPC(err) == codes.Unimplemented {
		orgs = &sourcegraph.OrgList{} // ignore error
	} else if err != nil {
		return *userSpec, nil, err
	}

	orgsAndSelf := []*sourcegraph.User{p}
	for _, org := range orgs.Orgs {
		orgsAndSelf = append(orgsAndSelf, &org.User)
	}

	// The current user can only view their own profile, as well as
	// the profiles of orgs they are an admin for.
	currentUserCanAdminOrg := false
	for _, adminable := range orgsAndSelf {
		if p.UID == adminable.UID {
			currentUserCanAdminOrg = true
			break
		}
	}
	if !currentUserCanAdminOrg {
		return *userSpec, nil, &errcode.HTTPErr{
			Status: http.StatusForbidden,
			Err:    errors.New("only a user or an org admin can view/edit profile"),
		}
	}

	return *userSpec, &userSettingsCommonData{
		User:        p,
		OrgsAndSelf: orgsAndSelf,
	}, nil
}

// Redirects "/.me/.settings/*" to "/<u>/.settings/*". Returns errUserSettingsCommonWroteResponse if redirect should
// happen. Otherwise returns nil.
func userSettingsMeRedirect(w http.ResponseWriter, r *http.Request, u *sourcegraph.UserSpec) error {
	if u == nil {
		return nil
	}

	vars := mux.Vars(r)
	if userSpec_, err := sourcegraph.ParseUserSpec(vars["User"]); err == nil && userSpec_.Login == ".me" {
		varsCopy := make(map[string]string)
		for k, v := range vars {
			varsCopy[k] = vars[v]
		}
		varsCopy["User"] = u.Login

		redirectURL := router.Rel.URLTo(httpctx.RouteName(r), router_util.MapToArray(varsCopy)...)
		http.Redirect(w, r, redirectURL.String(), http.StatusFound)
		return errUserSettingsCommonWroteResponse
	}
	return nil
}

func serveUserSettingsProfile(w http.ResponseWriter, r *http.Request) error {
	ctx, cl := handlerutil.Client(r)
	userSpec, cd, err := userSettingsCommon(w, r)
	if err == errUserSettingsCommonWroteResponse {
		return nil
	} else if err != nil {
		return err
	}

	emails, err := cl.Users.ListEmails(ctx, &userSpec)
	if err != nil || cd.User.IsOrganization {
		if grpc.Code(err) == codes.PermissionDenied || cd.User.IsOrganization {
			// We are not allowed to view the emails or its an org and orgs don't have emails
			// so just show an empty list
			emails = &sourcegraph.EmailAddrList{EmailAddrs: []*sourcegraph.EmailAddr{}}
		} else {
			return err
		}
	}

	if r.Method == "POST" {
		user := cd.User
		user.Name = r.PostFormValue("Name")
		if _, err := cl.Accounts.Update(ctx, user); err != nil {
			return err
		}

		http.Redirect(w, r, router.Rel.URLTo(router.UserSettingsProfile, "User", cd.User.Login).String(), http.StatusSeeOther)
		return nil
	}

	return tmpl.Exec(r, w, "user/settings/profile.html", http.StatusOK, nil, &struct {
		userSettingsCommonData
		tmpl.Common
		EmailAddrs []*sourcegraph.EmailAddr
	}{
		userSettingsCommonData: *cd,
		EmailAddrs:             emails.EmailAddrs,
	})
}

func serveUserSettingsProfileAvatar(w http.ResponseWriter, r *http.Request) error {
	ctx, cl := handlerutil.Client(r)

	_, cd, err := userSettingsCommon(w, r)
	if err == errUserSettingsCommonWroteResponse {
		return nil
	} else if err != nil {
		return err
	}

	user := cd.User
	email := r.PostFormValue("GravatarEmail")
	user.AvatarURL = gravatarURL(email)

	_, err = cl.Accounts.Update(ctx, user)
	if err != nil {
		return err
	}

	http.Redirect(w, r, router.Rel.URLTo(router.UserSettingsProfile, "User", user.Login).String(), http.StatusSeeOther)
	return nil
}

// gravatarURL returns the URL to the Gravatar avatar image for email.
// The generated URL can have a "&s=128"-like suffix appended to set the size.
// That allows it to be compatible with User.AvatarURLOfSize.
func gravatarURL(email string) string {
	email = strings.TrimSpace(email) // Trim leading and trailing whitespace from an email address.
	email = strings.ToLower(email)   // Force all characters to lower-case.
	h := md5.New()
	io.WriteString(h, email) // md5 hash the final string.
	return fmt.Sprintf("https://secure.gravatar.com/avatar/%x?d=mm", h.Sum(nil))
}

func serveUserSettingsKeys(w http.ResponseWriter, r *http.Request) error {
	_, cd, err := userSettingsCommon(w, r)
	if err == errUserSettingsCommonWroteResponse {
		return nil
	} else if err != nil {
		return err
	}

	return tmpl.Exec(r, w, "user/settings/keys.html", http.StatusOK, nil, &struct {
		userSettingsCommonData
		tmpl.Common
	}{
		userSettingsCommonData: *cd,
	})
}
