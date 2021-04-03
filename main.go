package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/jszwec/csvutil"
	"github.com/karashiiro/bingode"
	"github.com/xivapi/godestone/v2"
	"github.com/xivapi/godestone/v2/data/gender"
)

// The number of characters to attempt to fetch.
var characterCount uint32 = 35261910 / 10 // Highest as of April 1, 2021 2:52 PM PDT

// Number of goroutines to execute at once. Setting this too high will
// get you IP-blocked for a couple of days (can still log into the game).
// On non-residential IPs the rate limit seems to be significantly higher.
var parallelism uint32 = 20

// Number of characters to skip in iteration. Multiply this by
// the character count to get the maximum ID the program will attempt
// to fetch.
var sampleRate uint32 = 10

type Time struct {
	time.Time
}

const format = "2006/01/02 15:04:05"

func (t Time) MarshalCSV() ([]byte, error) {
	var b [len(format)]byte
	return t.AppendFormat(b[:0], format), nil
}

type CharacterInfo struct {
	ID                uint32 `csv:"id"`
	Name              string `csv:"name"`
	World             string `csv:"world"`
	Avatar            string `csv:"avatar"`
	ActiveClassJob    uint8  `csv:"active_classjob_id"`
	FreeCompany       string `csv:"free_company_id"`
	PVPTeam           string `csv:"pvp_team_id"`
	FirstAchievement  Time   `csv:"first_achievement"`
	Achievements      uint32 `csv:"achievements"`
	AchievementPoints uint32 `csv:"achievements"`
	Race              string `csv:"race"`
	Clan              string `csv:"clan"`
	Gender            string `csv:"gender"`
	City              string `csv:"starting_city"`
}

func stringifyGender(g gender.Gender) string {
	if g == gender.Male {
		return "Male"
	} else {
		return "Female"
	}
}

var idsLeft = characterCount
var lastRequestDuration time.Duration

func getCreationInfos(scraper *godestone.Scraper, ids chan uint32, done chan []*CharacterInfo) {
	creationInfo := make([]*CharacterInfo, 0)

	now := time.Now()
	for i := range ids {
		idsLeft--
		startTime := time.Now()
		c, err1 := scraper.FetchCharacter(i)
		if err1 == nil {
			if c.Race == nil {
				continue
			}

			currentCreationInfo := &CharacterInfo{
				ID:          i,
				Name:        c.Name,
				World:       c.World,
				Avatar:      c.Avatar,
				FreeCompany: c.FreeCompanyID,
				PVPTeam:     c.PvPTeamID,
				Race:        c.Race.NameFeminineEN,   // Masculine and feminine names are the same in English
				Clan:        c.Tribe.NameMasculineEN, // Same as above
				Gender:      stringifyGender(c.Gender),
				City:        c.Town.NameEN,
			}

			if c.ActiveClassJob != nil {
				currentCreationInfo.ActiveClassJob = c.ActiveClassJob.UnlockedState.ID
			}

			// Add achievement info, if possible
			acc, aai, err2 := scraper.FetchCharacterAchievements(i)
			if err2 == nil {
				oldest := now
				hasAny := false
				for _, a := range acc {
					if a.Date.Before(oldest) {
						oldest = a.Date
						hasAny = true
					}
				}

				if hasAny {
					currentCreationInfo.FirstAchievement = Time{oldest}
				}

				currentCreationInfo.Achievements = aai.TotalAchievements
				currentCreationInfo.AchievementPoints = aai.TotalAchievementPoints
			}

			creationInfo = append(creationInfo, currentCreationInfo)

			// Logging
			lastRequestDuration = time.Since(startTime)
			timeRemaining := lastRequestDuration.Seconds() * float64(idsLeft) / float64(parallelism)
			timeUnits := "seconds"
			if timeRemaining > 60 {
				timeRemaining /= 60
				timeUnits = "minutes"
				if timeRemaining > 60 {
					timeRemaining /= 60
					timeUnits = "hours"
					if timeRemaining > 24 {
						timeRemaining /= 24
						timeUnits = "days"
					}
				}
			}
			fmt.Println("ETA:", strconv.FormatFloat(timeRemaining, 'f', 6, 64), timeUnits, "remaining")
		}
	}
	done <- creationInfo
}

func main() {
	bin := bingode.New()
	scraper := godestone.NewScraper(bin, godestone.EN)

	charsPerGoroutine := characterCount / parallelism

	creationInfo := make([]*CharacterInfo, 0)
	creationInfoChans := make([]chan []*CharacterInfo, parallelism)
	for i := uint32(0); i < parallelism; i++ {
		idChan := make(chan uint32, charsPerGoroutine)
		creationInfoChans[i] = make(chan []*CharacterInfo, 1)

		go getCreationInfos(scraper, idChan, creationInfoChans[i])

		startID := uint32(1+i*charsPerGoroutine) * sampleRate
		endID := uint32((i+1)*charsPerGoroutine) * sampleRate

		for j := startID; j <= endID; j += sampleRate {
			idChan <- j
		}

		// Handle remainder
		if i == parallelism-1 {
			remainder := characterCount % parallelism
			endID += sampleRate
			for j := uint32(0); j < remainder; j++ {
				idChan <- endID + sampleRate*j
			}
		}

		close(idChan)
	}

	for i := uint32(0); i < parallelism; i++ {
		curCreationInfo := <-creationInfoChans[i]
		close(creationInfoChans[i])
		creationInfo = append(creationInfo, curCreationInfo...)
	}

	b, err := csvutil.Marshal(creationInfo)
	if err != nil {
		log.Fatalln(err)
	}

	f, err := os.Create("characters.csv")
	if err != nil {
		log.Fatalln(err)
	}

	defer f.Close()

	_, err = f.Write(b)
	if err != nil {
		log.Fatal(err)
	}
}
