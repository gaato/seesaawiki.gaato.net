package main

import (
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
)

type TemplateRenderer struct {
	templates *template.Template
}

func (t *TemplateRenderer) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	return t.templates.ExecuteTemplate(w, name, data)
}

func fetchOGData(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return "", err
	}

	var ogData string
	doc.Find("meta[property^='og:']").Each(func(i int, s *goquery.Selection) {
		property, _ := s.Attr("property")
		content, _ := s.Attr("content")
		ogData += fmt.Sprintf("%s: %s\n", property, content)
	})

	return ogData, nil
}

func main() {
	e := echo.New()

	renderer := &TemplateRenderer{
		templates: template.Must(template.ParseGlob("templates/*.html")),
	}

	e.Static("/static", "static")

	e.Renderer = renderer

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	// Routes
	e.GET("/", func(c echo.Context) error {
		inputUrl := c.QueryParam("url")

		if inputUrl == "" {
			return c.Render(http.StatusOK, "index.html", nil)
		}

		parsedUrl, err := url.Parse(inputUrl)
		if err != nil {
			return c.String(http.StatusBadRequest, fmt.Sprintf("Invalid url provided: %v", err))
		}

		eucJPPath, err := url.PathUnescape(parsedUrl.Path)
		if err != nil {
			return c.String(http.StatusBadRequest, fmt.Sprintf("Failed to unescape url path: %v", err))
		}

		// Convert EUC-JP to UTF-8
		utf8Reader := transform.NewReader(strings.NewReader(eucJPPath), japanese.EUCJP.NewDecoder())
		utf8Path, err := ioutil.ReadAll(utf8Reader)
		if err != nil {
			return c.String(http.StatusBadRequest, fmt.Sprintf("Failed to convert url path to utf-8: %v", err))
		}

		// Create new URL with the UTF-8 path
		parsedUrl.Path = string(utf8Path)

		// seesaawiki.jp を seesaawiki.gaato.net に置換
		parsedUrl.Host = strings.Replace(parsedUrl.Host, "seesaawiki.jp", "seesaawiki.gaato.net", 1)

		return c.Render(http.StatusOK, "index.html", map[string]interface{}{
			"InputUrl": inputUrl,
			"Url":      parsedUrl.String(),
		})
	})

	// Routes
	e.GET("/:encodedPath", func(c echo.Context) error {
		encodedPath := c.Param("encodedPath")

		utf8Path, err := url.PathUnescape(encodedPath)
		if err != nil {
			return c.String(http.StatusBadRequest, fmt.Sprintf("Failed to unescape url path: %v", err))
		}

		// Convert UTF-8 to EUC-JP
		eucJPReader := transform.NewReader(strings.NewReader(utf8Path), japanese.EUCJP.NewEncoder())
		eucJPPath, err := ioutil.ReadAll(eucJPReader)
		if err != nil {
			return c.String(http.StatusBadRequest, fmt.Sprintf("Failed to convert url path to EUC-JP: %v", err))
		}

		// Fetch OG data
		redirectUrl := "https://seesaawiki.jp/" + string(eucJPPath)
		resp, err := http.Get(redirectUrl)
		if err != nil {
			return c.String(http.StatusInternalServerError, fmt.Sprintf("Failed to fetch OG data: %v", err))
		}
		defer resp.Body.Close()

		doc, err := goquery.NewDocumentFromReader(resp.Body)
		if err != nil {
			return c.String(http.StatusInternalServerError, fmt.Sprintf("Failed to parse document: %v", err))
		}

		doc.Find("meta").Each(func(i int, s *goquery.Selection) {
			if property, exists := s.Attr("property"); exists {
				if strings.HasPrefix(property, "og:") {
					content, _ := s.Attr("content")
					c.Response().Header().Add(property, content)
				}
			}
		})

		// Redirect to the seesaawiki.jp URL with the EUC-JP path
		return c.Redirect(http.StatusTemporaryRedirect, redirectUrl)
	})
	// Start server
	e.Logger.Fatal(e.Start(":1323"))
}