// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

//revive:disable:get-return
package server

import (
	"context"
	"errors"
	"log/slog"
	"mime"
	"mime/multipart"
	"net/http"

	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"
	"github.com/davecgh/go-spew/spew"

	"github.com/joshuar/go-feed-me/internal/api"
	"github.com/joshuar/go-feed-me/internal/forms"
	"github.com/joshuar/go-feed-me/internal/logging"
	"github.com/joshuar/go-feed-me/internal/models"
	"github.com/joshuar/go-feed-me/internal/platforms/elastic"
	"github.com/joshuar/go-feed-me/internal/validation"
	"github.com/joshuar/go-feed-me/web/templates/partials/subscription"
)

type OPMLFile struct {
	data multipart.File
	hdr  *multipart.FileHeader
}

func (f *OPMLFile) Load(data multipart.File, hdr *multipart.FileHeader) error {
	f.data = data
	f.hdr = hdr
	return nil
}

func (f *OPMLFile) Valid() bool {
	mediaType, _, err := mime.ParseMediaType(f.hdr.Header.Get("Content-Type"))
	if err != nil {
		return false
	}
	return mediaType == "text/x-opml+xml"
}

// NewSubscription creates a new APISubscription request and presents it as a
// form for the user to fill out.
func (s Server) NewSubscription(res http.ResponseWriter, req *http.Request) {
	if err := htmx.NewResponse().RenderTempl(req.Context(), res, subscription.Modal(&api.SubscriptionRequest{})); err != nil {
		logging.FromContext(req.Context()).Error("Cannot display content.",
			slog.Any("error", errors.Join(ErrRenderTemplateFail, err)))
		http.Error(res, "Problem!", http.StatusInternalServerError)
	}
}

func (s Server) AddSubscription(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	user, found := models.UserFromCtx(ctx)
	if !found {
		logging.FromContext(ctx).Error("No user found?")
		http.Error(res, "Problem!", http.StatusInternalServerError)

		return
	}

	newSubscription, valid, err := forms.DecodeForm[*api.SubscriptionRequest](req)
	if err != nil {
		logging.FromContext(ctx).Error("Could not decode submitted subscription request request.",
			slog.Any("error", err))
		http.Error(res, "Problem!", http.StatusInternalServerError)

		return
	}

	if !valid {
		addSubscriptionError(ctx, res, newSubscription, "Please check your inputs and try again.")
		return
	}

	// Find any existing feed with the subscription URL or create a new feed.
	feedID, err := elastic.FindOrAddFeed(ctx, s.DataAPI().GetAPI(), newSubscription.URL)
	if err != nil {
		addSubscriptionError(ctx, res, newSubscription, "Unable to verify URL maps to a feed.")
		return
	}

	err = elastic.UserActionAddSubscription(ctx, s.DataAPI().GetAPI(), user, feedID, newSubscription)
	if err != nil {
		addSubscriptionError(ctx, res, newSubscription, "There was a temporary problem adding a subscription. Please try again.")
		return
	}

	if err := htmx.NewResponse().Retarget("#command_modal").RenderTempl(ctx, res, subscription.AddSubscriptionSuccess()); err != nil {
		logging.FromContext(ctx).Error("Cannot display content.",
			slog.Any("error", errors.Join(ErrRenderTemplateFail, err)))
		http.Error(res, "Problem!", http.StatusInternalServerError)
	}
}

func (s Server) StartImport(res http.ResponseWriter, req *http.Request) {
	if err := htmx.NewResponse().RenderTempl(req.Context(), res, subscription.ImportModal()); err != nil {
		logging.FromContext(req.Context()).Error("Cannot display content.",
			slog.Any("error", errors.Join(ErrRenderTemplateFail, err)))
		http.Error(res, "Problem!", http.StatusInternalServerError)
	}

	// w.WriteHeader(http.StatusNotImplemented)
}

func (f *SetImportMethodFormdataBody) Valid() bool {
	valid, err := validation.ValidateStruct(f)
	if !valid || err != nil {
		spew.Dump(err)
		return false
	}
	return true
}

func (s Server) SetImportMethod(res http.ResponseWriter, req *http.Request) {
	importMethod, valid, err := forms.DecodeForm[*SetImportMethodFormdataBody](req)
	switch {
	case err != nil:
		slog.Error("error parsing form", slog.Any("error", err))
	case !valid:
		slog.Error("invalid input")
	}

	var form templ.Component
	switch importMethod.From {
	case "opml_file":
		form = subscription.ImportFromOPML()
	}

	if err := htmx.NewResponse().RenderTempl(req.Context(), res, form); err != nil {
		logging.FromContext(req.Context()).Error("Cannot display content.",
			slog.Any("error", errors.Join(ErrRenderTemplateFail, err)))
		http.Error(res, "Problem!", http.StatusInternalServerError)
	}
}

func (s Server) ProcessImport(res http.ResponseWriter, req *http.Request) {
	opmlFile := &OPMLFile{}
	opmlFile, valid, err := forms.DecodeFile(req, "data", opmlFile)
	switch {
	case err != nil:
		slog.Error("error decoding file", slog.Any("error", err))
	case !valid:
		slog.Error("invalid file")
	}

	spew.Dump(opmlFile)
}

func (s Server) SaveSubscription(w http.ResponseWriter, r *http.Request, feedID models.FeedID) {
	w.WriteHeader(http.StatusNotImplemented)
}

func (s Server) ShowSubscription(w http.ResponseWriter, r *http.Request, feedID models.FeedID) {
	w.WriteHeader(http.StatusNotImplemented)
}

func (s Server) RemoveSubscription(w http.ResponseWriter, r *http.Request, feedID models.FeedID) {
	w.WriteHeader(http.StatusNotImplemented)
}

func (f *AddSubscriptionCategoryFormdataRequestBody) Valid() bool {
	return f.Category != ""
}

func (s Server) AddSubscriptionCategory(res http.ResponseWriter, req *http.Request) {
	var response templ.Component

	data, valid, err := forms.DecodeForm[*AddSubscriptionCategoryFormdataRequestBody](req)
	if !valid {
		response = subscription.AddSubscriptionWarning("Invalid ")
	} else {
		response = subscription.AddCategory(data.Category)
	}

	if err != nil {
		logging.FromContext(req.Context()).Error("Cannot display content.",
			slog.Any("error", errors.Join(ErrRenderTemplateFail, err)))
	}

	if err := htmx.NewResponse().RenderTempl(req.Context(), res, response); err != nil {
		logging.FromContext(req.Context()).Error("Cannot display content.",
			slog.Any("error", errors.Join(ErrRenderTemplateFail, err)))
		http.Error(res, "Problem!", http.StatusInternalServerError)
	}
}

func (s Server) RemoveSubscriptionCategory(res http.ResponseWriter, req *http.Request) {
	res.WriteHeader(http.StatusOK)

	if _, err := res.Write(nil); err != nil {
		logging.FromContext(req.Context()).Error("Cannot display content.",
			slog.Any("error", errors.Join(ErrRenderTemplateFail, err)))
		http.Error(res, "Problem!", http.StatusInternalServerError)
	}
}

func addSubscriptionError(ctx context.Context, res http.ResponseWriter, details *api.SubscriptionRequest, message string) {
	var err error
	resp := htmx.NewResponse()

	if err = resp.RenderTempl(ctx, res, subscription.AddSubscriptionWarning(message)); err != nil {
		logging.FromContext(ctx).Error("Cannot display content.",
			slog.Any("error", errors.Join(ErrRenderTemplateFail, err)))
		http.Error(res, "Problem!", http.StatusInternalServerError)
	}

	if err = resp.RenderTempl(ctx, res, subscription.Form(details)); err != nil {
		logging.FromContext(ctx).Error("Cannot display content.",
			slog.Any("error", errors.Join(ErrRenderTemplateFail, err)))
		http.Error(res, "Problem!", http.StatusInternalServerError)
	}
}
