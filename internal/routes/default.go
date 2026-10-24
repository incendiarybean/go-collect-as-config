package routes

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/incendiarybean/go-collect-as-config/internal/collectors"
)

func DefaultRoutes(db *sql.DB) {
	router := http.NewServeMux()

	// Generic routing to display data
	router.Handle("GET /", http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		rows, err := db.Query("SELECT * FROM articles;")
		if err != nil {
			slog.Error("Could not query table.")
			http.Error(response, http.StatusText(http.StatusBadGateway), http.StatusBadGateway)
			return
		}
		defer func() {
			_ = rows.Close()
		}()

		var articles []collectors.ArticleInner

		for rows.Next() {
			var article collectors.ArticleInner

			if err := rows.Scan(&article.Id, &article.Hash, &article.Title, &article.Href, &article.Img, &article.Timestamp, &article.Outlet, &article.Description); err != nil {
				slog.Error(err.Error())
				http.Error(response, http.StatusText(http.StatusBadGateway), http.StatusBadGateway)
				return
			}
			articles = append(articles, article)
		}

		if err = rows.Err(); err != nil {
			slog.Error(err.Error())
			http.Error(response, http.StatusText(http.StatusBadGateway), http.StatusBadGateway)
			return
		}

		response.Header().Set("content-type", "application/json")

		_ = json.NewEncoder(response).Encode(articles)
	}))

	_ = http.ListenAndServe(":8080", router)
}
