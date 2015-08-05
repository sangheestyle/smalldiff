package main

import (
	"fmt"
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
	CreatedAt       time.Time
	PushedAt        time.Time
	UpdatedAt       time.Time
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

	mux.GET("/crawl/github/:name", DoCrawlGithubRepos)

	log.Fatal(http.ListenAndServe(":8080", mux))
}

// Hello shows name
func DoCrawlGithubRepos(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	if p.ByName("name") == "repos" {
		fmt.Fprintf(w, "Crawl, %s!\n", p.ByName("name"))
		var query string
		dates, err := GenerateDates("2015-01-01", "2015-01-02")
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
	} else {
		fmt.Fprintf(w, "Not Crawl, %s!\n", p.ByName("name"))
	}
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
			repo := Repo{
				ID:              r.ID,
				FullName:        r.FullName,
				CreatedAt:       r.CreatedAt.Time,
				PushedAt:        r.PushedAt.Time,
				UpdatedAt:       r.UpdatedAt.Time,
				GitURL:          r.GitURL,
				Langauge:        r.Language,
				ForksCount:      r.ForksCount,
				OpenIssuesCount: r.OpenIssuesCount,
				WatchersCount:   r.WatchersCount,
				Size:            r.Size,
			}

			// Create new and update existed one
			if Db.Where("ID = ?", repo.ID).First(&repo).RecordNotFound() {
				created += 1
				Db.Create(&repo)
			} else {
				updated += 1
				Db.Save(&repo)
			}
		}

		if resp.NextPage == 0 {
			break
		}

		opts.ListOptions.Page = resp.NextPage
	}

	return created, updated, err
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
