package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"time"

	"github.com/google/go-github/github"
	"github.com/jinzhu/gorm"
	"github.com/julienschmidt/httprouter"
	_ "github.com/lib/pq"
)

type Repo struct {
	ID              *int
	Name            *string
	FullName        *string
	GCreatedAt      time.Time
	GPushedAt       time.Time
	GUpdatedAt      time.Time
	GitURL          *string
	Langauge        *string
	ForksCount      *int
	OpenIssuesCount *int
	WatchersCount   *int
	Size            *int
}

var Db gorm.DB

func init() {
	var err error
	Db, err = gorm.Open("postgres",
		"user=smalldiff dbname=smalldiff password=smalldiff sslmode=disable")
	if err != nil {
		panic(err)
	}
	Db.AutoMigrate(&Repo{})
}

func main() {
	mux := httprouter.New()

	mux.GET("/crawl/github/repos", DoCrawlGithubReposForm)
	mux.POST("/crawl/github/repos", DoCrawlGithubRepos)

	log.Fatal(http.ListenAndServe(":8080", mux))
}

// DoCrawlGithubReposForm responses from to crawl github repos
func DoCrawlGithubReposForm(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	t, _ := template.ParseFiles("crawl_github_repos_form.html")
	t.Execute(w, nil)
}

// DoCrawlGithubRepos starts to crawl github repos
func DoCrawlGithubRepos(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	if r.FormValue("password") != "repos" {
		fmt.Fprintf(w, "Can't crawl due to wrong password!\n")
		return
	}

	go func() {
		var query string
		dates, err := GenerateDates(r.FormValue("start_date"), r.FormValue("end_date"))
		if err != nil {
			fmt.Println(err)
		}
		for _, date := range dates {
			query = "android in:name,description,readme created:" + date
			created, updated, err := CrawlGithubRepos(query)
			if err != nil {
				fmt.Println(err)
			}
			fmt.Printf("DoCrawlGithubRepos: Created: %d, Updated: %d, %s\n",
				created, updated, date)
		}
	}()
	fmt.Fprintf(w, "Doing crawl at background!\n")
}

// CrawlGithubRepos searches repos and stores them into db
func CrawlGithubRepos(query string) (created int, updated int, err error) {
	client := github.NewClient(nil)
	opts := &github.SearchOptions{Sort: "forks", Order: "desc",
		ListOptions: github.ListOptions{PerPage: 100}}

	for {
		// Check RateLimit for search
		rate, _, _ := client.RateLimits()
		if rate.Search.Remaining == 0 {
			fmt.Printf("CrawlGithubRepos: wait till %v\n", rate.Search.Reset)
			duration := rate.Search.Reset.Time.Sub(time.Now()) + time.Second*5
			time.Sleep(duration)
		}

		repos, resp, err := client.Search.Repositories(query, opts)
		if err != nil {
			return created, updated, err
		}

		for _, r := range repos.Repositories {
			r_created, r_updated := StoreGithubRepo(r)
			created += r_created
			updated += r_updated
		}

		if resp.NextPage == 0 {
			break
		}

		opts.ListOptions.Page = resp.NextPage
	}

	return created, updated, err
}

// StoreGithubRepo store a repo into DB
func StoreGithubRepo(r github.Repository) (created int, updated int) {
	// CreatedAt with nil makes panic
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in repos", r)
		}
	}()
	repo := &Repo{
		ID:              r.ID,
		FullName:        r.FullName,
		GCreatedAt:      r.CreatedAt.Time,
		GPushedAt:       r.PushedAt.Time,
		GUpdatedAt:      r.UpdatedAt.Time,
		GitURL:          r.GitURL,
		Langauge:        r.Language,
		ForksCount:      r.ForksCount,
		OpenIssuesCount: r.OpenIssuesCount,
		WatchersCount:   r.WatchersCount,
		Size:            r.Size,
	}

	// Create new and update existed one
	if Db.Where("ID = ?", repo.ID).First(repo).RecordNotFound() {
		created += 1
		Db.Create(repo)
	} else {
		updated += 1
		Db.Save(repo)
	}
	return
}

// GenerateDates retures dates between begin and end date
func GenerateDates(begin string, end string) (dates []string, err error) {
	var t1 time.Time
	var t2 time.Time
	layout := "2006-01-02"

	t1, err = time.Parse(layout, begin)
	if err != nil {
		return nil, err
	}
	t2, err = time.Parse(layout, end)
	if err != nil {
		return nil, err
	}
	for {
		if t1.After(t2) {
			break
		}
		dates = append(dates, t1.String()[:10])
		t1 = t1.Add(time.Hour * 24)
	}
	return
}
