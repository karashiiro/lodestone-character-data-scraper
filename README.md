# lodestone-character-data-scraper
Lodestone character data scraper. Builds a CSV with general information about a large sample of characters created before April 1, 2021 2:52 PM PDT.
For the moment, if you would like more specific information about characters, you can fork this application and add fields to the row object.

It's currently configured to scrape every 100th character since the game's release, and takes about a day to run.

## Building
`make build`

## Running
`make scrape`