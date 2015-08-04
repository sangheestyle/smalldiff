package main

import (
	"fmt"
	"time"

	"github.com/google/go-github/github"
	"github.com/jinzhu/gorm"
	_ "github.com/lib/pq"
)

type Repo struct {
	ID              *int
	Name            *string
	FullName        *string
	CreatedAt       *github.Timestamp
	PushedAt        *github.Timestamp
	UpdatedAt       *github.Timestamp
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
	query := "android in:name,description,readme created:2014-01-01"
	total, err := CrawlGithubRepos(query)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Printf("Total: %d", total)
}

// CrawlGithubRepos searches repos and stores them into db
func CrawlGithubRepos(query string) (total int, err error) {
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
			return total, err
		}

		total += len(repos.Repositories)
		for _, r := range repos.Repositories {
			repo := Repo{
				ID:              r.ID,
				FullName:        r.FullName,
				CreatedAt:       r.CreatedAt,
				PushedAt:        r.PushedAt,
				UpdatedAt:       r.UpdatedAt,
				GitURL:          r.GitURL,
				Langauge:        r.Language,
				ForksCount:      r.ForksCount,
				OpenIssuesCount: r.OpenIssuesCount,
				WatchersCount:   r.WatchersCount,
				Size:            r.Size,
			}
			Db.Save(&repo)
		}

		if resp.NextPage == 0 {
			break
		}

		opts.ListOptions.Page = resp.NextPage
	}

	return total, err
}
