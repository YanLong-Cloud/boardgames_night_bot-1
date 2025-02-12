package database

import (
	"boardgame-night-bot/src/models"
	"database/sql"
	"errors"
	"log"
	"sort"

	"github.com/jackc/pgx/v5/pgtype"
	_ "github.com/mattn/go-sqlite3"
)

type Database struct {
	db *sql.DB
}

var ErrNoRows = errors.New("sql: no rows in result set")

func NewDatabase() *Database {
	db, err := sql.Open("sqlite3", "bot_data.sqlite")
	if err != nil {
		log.Fatal(err)
	}

	return &Database{db}
}

func NamedArgs(arg map[string]any) []any {
	args := make([]any, 0, len(arg))
	for k, v := range arg {
		args = append(args, sql.Named(k, v))
	}

	return args
}

func (d *Database) CreateTables() {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			chat_id INTEGER,
			user_id INTEGER,
			user_name TEXT,
			name TEXT,
			message_id INTEGER
		);`,
		`CREATE TABLE IF NOT EXISTS boardgames (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			event_id INTEGER,
			name TEXT,
			max_players INTEGER,
			message_id INTEGER,
			bgg_id INTEGER,
			bgg_name TEXT,
			bgg_url TEXT,
			FOREIGN KEY(event_id) REFERENCES events(id) ON DELETE CASCADE
		);`,
		`CREATE TABLE IF NOT EXISTS participants (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			event_id INTEGER,
			boardgame_id INTEGER,
			user_id INTEGER,
			user_name TEXT,
			FOREIGN KEY(event_id) REFERENCES boardgames(events) ON DELETE CASCADE,
			FOREIGN KEY(boardgame_id) REFERENCES boardgames(id) ON DELETE CASCADE,
			UNIQUE(event_id, user_id) ON CONFLICT REPLACE
		);`,
	}

	for _, query := range queries {
		_, err := d.db.Exec(query)
		if err != nil {
			log.Fatal(err)
		}
	}

	log.Println("Database tables ensured.")
}

func (d *Database) Close() {
	d.db.Close()
	log.Println("Database connection closed.")
}

func (d *Database) InsertEvent(chatID, userID int64, userName, name string, messageID *int64) (int64, error) {
	var eventID int64
	query := `INSERT INTO events (chat_id, user_id, user_name, name, message_id) VALUES (@chat_id, @user_id, @user_name, @name, @message_id) RETURNING id;`

	if err := d.db.QueryRow(query,
		NamedArgs(map[string]any{
			"chat_id":    chatID,
			"user_id":    userID,
			"user_name":  userName,
			"name":       name,
			"message_id": messageID,
		})...,
	).Scan(&eventID); err != nil {
		return 0, err
	}

	return eventID, nil
}

func (d *Database) SelectEvent(chatID int64) (*models.Event, error) {
	query := `
	SELECT 
	e.id, 
	e.name, 
	e.message_id,
	b.id,
	b.name,
	b.max_players,
	b.bgg_id,
	b.bgg_name,
	b.bgg_url,
	p.id,
	p.user_id,
	p.user_name
	FROM events e
	LEFT JOIN boardgames b ON e.id = b.event_id
	LEFT JOIN participants p ON b.id = p.boardgame_id
	WHERE e.id = (SELECT MAX(id) FROM events WHERE chat_id = @chat_id LIMIT 1);`

	rows, err := d.db.Query(query, NamedArgs(map[string]any{"chat_id": chatID})...)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	boardGameMap := make(map[int64]*models.BoardGame)

	event := &models.Event{}
	for rows.Next() {
		var boardGame models.BoardGame
		var participant models.Participant

		var eventMessageID, boardGameID, boardGameMaxPlayers, participantID, participantUserID, bggID pgtype.Int8
		var boardGameName, participantUserName, bggName, bggUrl pgtype.Text

		if err := rows.Scan(
			&event.ID,
			&event.Name,
			&eventMessageID,
			&boardGameID,
			&boardGameName,
			&boardGameMaxPlayers,
			&bggID,
			&bggName,
			&bggUrl,
			&participantID,
			&participantUserID,
			&participantUserName,
		); err != nil {
			return nil, err
		}

		event.MessageID = IntOrNil(eventMessageID)

		if IntOrNil(boardGameID) != nil {
			boardGame = models.BoardGame{
				ID:         *IntOrNil(boardGameID),
				Name:       *StringOrNil(boardGameName),
				MaxPlayers: *IntOrNil(boardGameMaxPlayers),
				BggID:      IntOrNil(bggID),
				BggName:    StringOrNil(bggName),
				BggUrl:     StringOrNil(bggUrl),
			}

			if _, ok := boardGameMap[boardGame.ID]; !ok {
				boardGameMap[boardGame.ID] = &boardGame
			}
		}

		if IntOrNil(participantID) != nil {
			participant = models.Participant{
				ID:       *IntOrNil(participantID),
				UserID:   *IntOrNil(participantUserID),
				UserName: *StringOrNil(participantUserName),
			}

			boardGameMap[boardGame.ID].Participants = append(boardGame.Participants, participant)
		}
	}

	for _, boardGame := range boardGameMap {
		sort.SliceStable(boardGame.Participants, func(i, j int) bool {
			return boardGame.Participants[i].UserName < boardGame.Participants[j].UserName
		})

		event.BoardGames = append(event.BoardGames, *boardGame)
	}

	sort.SliceStable(event.BoardGames, func(i, j int) bool {
		return event.BoardGames[i].Name < event.BoardGames[j].Name || (event.BoardGames[i].Name == event.BoardGames[j].Name && event.BoardGames[i].ID < event.BoardGames[j].ID)
	})

	return event, nil
}

func (d *Database) UpdateEventMessageID(eventID, messageID int64) error {
	query := `UPDATE events SET message_id = @message_id where id = @event_id;`

	if _, err := d.db.Exec(query,
		NamedArgs(map[string]any{
			"event_id":   eventID,
			"message_id": messageID,
		})...,
	); err != nil {
		return err
	}

	return nil
}

func (d *Database) InsertBoardGame(eventID int64, name string, maxPlayers int, bggID *int64, bggName, bggUrl *string) (int64, error) {
	var boardGameID int64
	query := `INSERT INTO boardgames (event_id, name, max_players, bgg_id, bgg_name, bgg_url) VALUES (@event_id, @name, @max_players, @bgg_id, @bgg_name, @bgg_url) RETURNING id;`

	if err := d.db.QueryRow(query,
		NamedArgs(map[string]any{
			"event_id":    eventID,
			"name":        name,
			"max_players": maxPlayers,
			"bgg_id":      bggID,
			"bgg_url":     bggUrl,
			"bgg_name":    bggName,
		})...,
	).Scan(&boardGameID); err != nil {
		return 0, err
	}

	return boardGameID, nil
}

func (d *Database) UpdateBoardGameMessageID(boardgameID, messageID int64) error {
	query := `UPDATE boardgames SET message_id = @message_id where id = @boardgame_id;`

	if _, err := d.db.Exec(query,
		NamedArgs(map[string]any{
			"boardgame_id": boardgameID,
			"message_id":   messageID,
		})...,
	); err != nil {
		return err
	}

	return nil
}

func (d *Database) UpdateBoardGamePlayerNumber(messageID int64, maxPlayers int) error {
	var boardGameID int64

	query := `UPDATE boardgames SET max_players = @max_players where message_id = @message_id RETURNING id;`

	if err := d.db.QueryRow(query,
		NamedArgs(map[string]any{
			"max_players": maxPlayers,
			"message_id":  messageID,
		})...,
	).Scan(&boardGameID); err != nil {
		return ParseError(err)
	}

	return nil
}

func (d *Database) InsertParticipant(eventID int64, boardgameID, userID int64, userName string) (int64, error) {
	var participantID int64
	query := `INSERT INTO participants (event_id, boardgame_id, user_id, user_name) VALUES (@event_id, @boardgame_id, @user_id, @user_name) RETURNING id;`

	if err := d.db.QueryRow(query,
		NamedArgs(map[string]any{
			"event_id":     eventID,
			"boardgame_id": boardgameID,
			"user_id":      userID,
			"user_name":    userName,
		})...,
	).Scan(&participantID); err != nil {
		return 0, err
	}

	return participantID, nil
}

func (d *Database) RemoveParticipant(eventID int64, userID int64) error {
	query := `DELETE FROM participants WHERE event_id = @event_id AND user_id = @user_id;`

	if _, err := d.db.Exec(query,
		NamedArgs(map[string]any{
			"event_id": eventID,
			"user_id":  userID,
		})...,
	); err != nil {
		return err
	}

	return nil
}

func IntOrNil(i pgtype.Int8) *int64 {
	if i.Valid {
		v := i.Int64
		return &v
	}

	return nil
}

func StringOrNil(i pgtype.Text) *string {
	if i.Valid {
		v := i.String
		return &v
	}

	return nil
}

func ParseError(err error) error {
	if err.Error() == "sql: no rows in result set" {
		return ErrNoRows
	}

	return err
}
