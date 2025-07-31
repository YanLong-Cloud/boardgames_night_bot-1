package database

import (
	"boardgame-night-bot/src/models"
	"database/sql"
	"errors"
	"log"
	"path/filepath"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	_ "github.com/mattn/go-sqlite3"
)

type Database struct {
	db *sql.DB
}

var ErrNoRows = errors.New("sql: no rows in result set")

func NewDatabase(path string) *Database {
	db, err := sql.Open("sqlite3", filepath.Join(path, "bot_data.sqlite"))
	if err != nil {
		log.Fatal("failed to open database '"+filepath.Join(path, "bot_data.sqlite")+"':", err)
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
		`CREATE TABLE IF NOT EXISTS chats (
			chat_id INTEGER NOT NULL,
			language TEXT NOT NULL DEFAULT 'en',
			PRIMARY KEY(chat_id)
			UNIQUE(chat_id) ON CONFLICT REPLACE
		);`,
		`CREATE TABLE IF NOT EXISTS events (
			id TEXT PRIMARY KEY,
			chat_id INTEGER,
			user_id INTEGER,
			user_name TEXT,
			name TEXT,
			message_id INTEGER,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
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
			bgg_image_url TEXT,
			FOREIGN KEY(event_id) REFERENCES events(id) ON DELETE CASCADE
   			FOREIGN KEY(initiator_name) REFERENCES events(user_name) ON DELETE CASCADE
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

	log.Println("database tables ensured")
}

func (d *Database) Close() {
	d.db.Close()
	log.Println("database connection closed")
}

func (d *Database) InsertEvent(chatID, userID int64, userName, name string, messageID *int64) (string, error) {
	var eventID string
	query := `INSERT INTO events (id, chat_id, user_id, user_name, name, message_id) VALUES (@event_id, @chat_id, @user_id, @user_name, @name, @message_id) RETURNING id;`

	if err := d.db.QueryRow(query,
		NamedArgs(map[string]any{
			"event_id":   uuid.New().String(),
			"chat_id":    chatID,
			"user_id":    userID,
			"user_name":  userName,
			"name":       name,
			"message_id": messageID,
		})...,
	).Scan(&eventID); err != nil {
		return "", err
	}

	return eventID, nil
}

func (d *Database) SelectEvent(chatID int64) (*models.Event, error) {
	query := `
	SELECT 
	e.id, 
	e.name, 
	e.chat_id,
	e.message_id,
	e.user_id,
	b.id,
	b.name,
	b.max_players,
	b.bgg_id,
	b.bgg_name,
	b.bgg_url,
	b.bgg_image_url,
	p.id,
	p.user_id,
	p.user_name
	FROM events e
	LEFT JOIN boardgames b ON e.id = b.event_id
	LEFT JOIN participants p ON b.id = p.boardgame_id
	WHERE e.id = (SELECT id FROM events WHERE chat_id = @chat_id ORDER BY created_at DESC LIMIT 1);`
	return d.selectEventByQuery(query, map[string]any{"chat_id": chatID})
}

func (d *Database) SelectEventByEventID(eventID string) (*models.Event, error) {
	query := `
	SELECT 
	e.id, 
	e.name, 
	e.chat_id,
	e.message_id,
	e.user_id,
	b.id,
	b.name,
	b.max_players,
	b.bgg_id,
	b.bgg_name,
	b.bgg_url,
	b.bgg_image_url,
	p.id,
	p.user_id,
	p.user_name
	FROM events e
	LEFT JOIN boardgames b ON e.id = b.event_id
	LEFT JOIN participants p ON b.id = p.boardgame_id
	WHERE e.id = @id;`
	return d.selectEventByQuery(query, map[string]any{"id": eventID})
}

func (d *Database) selectEventByQuery(query string, args map[string]any) (*models.Event, error) {
	rows, err := d.db.Query(query, NamedArgs(args)...)
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
		var boardGameName, participantUserName, bggName, bggUrl, bggImageUrl pgtype.Text

		if err := rows.Scan(
			&event.ID,
			&event.Name,
			&event.ChatID,
			&eventMessageID,
			&event.UserID,
			&boardGameID,
			&boardGameName,
			&boardGameMaxPlayers,
			&bggID,
			&bggName,
			&bggUrl,
			&bggImageUrl,
			&participantID,
			&participantUserID,
			&participantUserName,
		); err != nil {
			return nil, err
		}

		event.MessageID = IntOrNil(eventMessageID)
		event.Locked = strings.Contains(event.Name, "🔒")

		if IntOrNil(boardGameID) != nil {
			boardGame = models.BoardGame{
				ID:          *IntOrNil(boardGameID),
				Name:        *StringOrNil(boardGameName),
				MaxPlayers:  *IntOrNil(boardGameMaxPlayers),
				BggID:       IntOrNil(bggID),
				BggName:     StringOrNil(bggName),
				BggUrl:      StringOrNil(bggUrl),
				BggImageUrl: StringOrNil(bggImageUrl),
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

			boardGameMap[boardGame.ID].Participants = append(boardGameMap[boardGame.ID].Participants, participant)
		}
	}

	if rows.Err() != nil {
		return nil, rows.Err()
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

func (d *Database) UpdateEventMessageID(eventID string, messageID int64) error {
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

func (d *Database) InsertBoardGame(eventID string, name string, maxPlayers int, bggID *int64, bggName, bggUrl, bggImageUrl *string, initiator_name string) (int64, error) {
	var boardGameID int64
	query := `INSERT INTO boardgames (event_id, name, max_players, bgg_id, bgg_name, bgg_url, bgg_image_url, initiator_name) VALUES (@event_id, @name, @max_players, @bgg_id, @bgg_name, @bgg_url, @bgg_image_url, @initiator_name) RETURNING id;`

	if bggImageUrl != nil && *bggImageUrl == "" {
		// Fix for BGG image URLs that contains a filter with mandatory (png)
		tmp := *bggImageUrl
		tmp = strings.ReplaceAll(tmp, "%28", "(")
		tmp = strings.ReplaceAll(tmp, "%29", ")")
		bggImageUrl = &tmp
	}

	if err := d.db.QueryRow(query,
		NamedArgs(map[string]any{
			"event_id":      eventID,
			"name":          name,
			"max_players":   maxPlayers,
			"bgg_id":        bggID,
			"bgg_url":       bggUrl,
			"bgg_name":      bggName,
			"bgg_image_url": bggImageUrl,
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

func (d *Database) UpdateBoardGameBGGInfo(messageID int64, maxPlayers int, bggID *int64, bggName, bggUrl, bggImageUrl *string) error {
	var boardGameID int64

	query := `UPDATE boardgames 
	SET 
	max_players = @max_players,
	bgg_id = @bgg_id,
	bgg_name = @bgg_name,
	bgg_url = @bgg_url,
	bgg_image_url = @bgg_image_url
	WHERE message_id = @message_id RETURNING id;`

	if err := d.db.QueryRow(query,
		NamedArgs(map[string]any{
			"max_players":   maxPlayers,
			"message_id":    messageID,
			"bgg_id":        bggID,
			"bgg_name":      bggName,
			"bgg_url":       bggUrl,
			"bgg_image_url": bggImageUrl,
		})...,
	).Scan(&boardGameID); err != nil {
		return ParseError(err)
	}

	return nil
}

func (d *Database) UpdateBoardGameBGGInfoByID(ID int64, maxPlayers int, bggID *int64, bggName, bggUrl, bggImageUrl *string) error {
	query := `UPDATE boardgames 
	SET 
	max_players = @max_players,
	bgg_id = @bgg_id,
	bgg_name = @bgg_name,
	bgg_url = @bgg_url,
	bgg_image_url = @bgg_image_url
	WHERE id = @id RETURNING id;`

	if err := d.db.QueryRow(query,
		NamedArgs(map[string]any{
			"max_players":   maxPlayers,
			"id":            ID,
			"bgg_id":        bggID,
			"bgg_name":      bggName,
			"bgg_url":       bggUrl,
			"bgg_image_url": bggImageUrl,
		})...,
	).Scan(&ID); err != nil {
		return ParseError(err)
	}

	return nil
}

func (d *Database) DeleteBoardGameByID(ID int64) error {
	query := `DELETE FROM boardgames WHERE id = @id;`

	if _, err := d.db.Exec(query,
		NamedArgs(map[string]any{
			"id": ID,
		})...,
	); err != nil {
		return err
	}

	return nil
}

func (d *Database) HasBoardGameWithMessageID(messageID int64) bool {
	query := `SELECT id FROM boardgames WHERE message_id = @message_id;`

	var id int64
	if err := d.db.QueryRow(query,
		NamedArgs(map[string]any{
			"message_id": messageID,
		})...,
	).Scan(&id); err != nil {
		return false
	}

	return true
}

func (d *Database) InsertParticipant(eventID string, boardgameID, userID int64, userName string) (int64, error) {
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

func (d *Database) RemoveParticipant(eventID string, userID int64) error {
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

func (d *Database) InsertChat(chatID int64, language string) error {
	query := `
		INSERT INTO chats (chat_id, language) 
		VALUES (@chat_id, @language)
		ON CONFLICT (chat_id) 
		DO UPDATE SET language = EXCLUDED.language;
	`

	if _, err := d.db.Exec(query,
		NamedArgs(map[string]any{
			"chat_id":  chatID,
			"language": language,
		})...,
	); err != nil {
		return err
	}

	return nil
}

func (d *Database) GetPreferredLanguage(chatID int64) string {
	query := `SELECT language FROM chats WHERE chat_id = @chat_id;`

	var language string
	if err := d.db.QueryRow(query,
		NamedArgs(map[string]any{
			"chat_id": chatID,
		})...,
	).Scan(&language); err != nil {
		return "en"
	}

	return language
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
}package database

import (
	"boardgame-night-bot/src/models"
	"database/sql"
	"errors"
	"log"
	"path/filepath"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	_ "github.com/mattn/go-sqlite3"
)

type Database struct {
	db *sql.DB
}

var ErrNoRows = errors.New("sql: no rows in result set")

func NewDatabase(path string) *Database {
	db, err := sql.Open("sqlite3", filepath.Join(path, "bot_data.sqlite"))
	if err != nil {
		log.Fatal("failed to open database '"+filepath.Join(path, "bot_data.sqlite")+"':", err)
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
		`CREATE TABLE IF NOT EXISTS chats (
			chat_id INTEGER NOT NULL,
			language TEXT NOT NULL DEFAULT 'en',
			PRIMARY KEY(chat_id)
			UNIQUE(chat_id) ON CONFLICT REPLACE
		);`,
		`CREATE TABLE IF NOT EXISTS events (
			id TEXT PRIMARY KEY,
			chat_id INTEGER,
			user_id INTEGER,
			user_name TEXT,
			name TEXT,
			message_id INTEGER,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
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
			bgg_image_url TEXT,
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

	log.Println("database tables ensured")
}

func (d *Database) Close() {
	d.db.Close()
	log.Println("database connection closed")
}

func (d *Database) InsertEvent(chatID, userID int64, userName, name string, messageID *int64) (string, error) {
	var eventID string
	query := `INSERT INTO events (id, chat_id, user_id, user_name, name, message_id) VALUES (@event_id, @chat_id, @user_id, @user_name, @name, @message_id) RETURNING id;`

	if err := d.db.QueryRow(query,
		NamedArgs(map[string]any{
			"event_id":   uuid.New().String(),
			"chat_id":    chatID,
			"user_id":    userID,
			"user_name":  userName,
			"name":       name,
			"message_id": messageID,
		})...,
	).Scan(&eventID); err != nil {
		return "", err
	}

	return eventID, nil
}

func (d *Database) SelectEvent(chatID int64) (*models.Event, error) {
	query := `
	SELECT 
	e.id, 
	e.name, 
	e.chat_id,
	e.message_id,
	e.user_id,
	b.id,
	b.name,
	b.max_players,
	b.bgg_id,
	b.bgg_name,
	b.bgg_url,
	b.bgg_image_url,
	p.id,
	p.user_id,
	p.user_name
	FROM events e
	LEFT JOIN boardgames b ON e.id = b.event_id
	LEFT JOIN participants p ON b.id = p.boardgame_id
	WHERE e.id = (SELECT id FROM events WHERE chat_id = @chat_id ORDER BY created_at DESC LIMIT 1);`
	return d.selectEventByQuery(query, map[string]any{"chat_id": chatID})
}

func (d *Database) SelectEventByEventID(eventID string) (*models.Event, error) {
	query := `
	SELECT 
	e.id, 
	e.name, 
	e.chat_id,
	e.message_id,
	e.user_id,
	b.id,
	b.name,
	b.max_players,
	b.bgg_id,
	b.bgg_name,
	b.bgg_url,
	b.bgg_image_url,
	p.id,
	p.user_id,
	p.user_name
	FROM events e
	LEFT JOIN boardgames b ON e.id = b.event_id
	LEFT JOIN participants p ON b.id = p.boardgame_id
	WHERE e.id = @id;`
	return d.selectEventByQuery(query, map[string]any{"id": eventID})
}

func (d *Database) selectEventByQuery(query string, args map[string]any) (*models.Event, error) {
	rows, err := d.db.Query(query, NamedArgs(args)...)
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
		var boardGameName, participantUserName, bggName, bggUrl, bggImageUrl pgtype.Text

		if err := rows.Scan(
			&event.ID,
			&event.Name,
			&event.ChatID,
			&eventMessageID,
			&event.UserID,
			&boardGameID,
			&boardGameName,
			&boardGameMaxPlayers,
			&bggID,
			&bggName,
			&bggUrl,
			&bggImageUrl,
			&participantID,
			&participantUserID,
			&participantUserName,
		); err != nil {
			return nil, err
		}

		event.MessageID = IntOrNil(eventMessageID)
		event.Locked = strings.Contains(event.Name, "🔒")

		if IntOrNil(boardGameID) != nil {
			boardGame = models.BoardGame{
				ID:          *IntOrNil(boardGameID),
				Name:        *StringOrNil(boardGameName),
				MaxPlayers:  *IntOrNil(boardGameMaxPlayers),
				BggID:       IntOrNil(bggID),
				BggName:     StringOrNil(bggName),
				BggUrl:      StringOrNil(bggUrl),
				BggImageUrl: StringOrNil(bggImageUrl),
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

			boardGameMap[boardGame.ID].Participants = append(boardGameMap[boardGame.ID].Participants, participant)
		}
	}

	if rows.Err() != nil {
		return nil, rows.Err()
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

func (d *Database) UpdateEventMessageID(eventID string, messageID int64) error {
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

func (d *Database) InsertBoardGame(eventID string, name string, maxPlayers int, bggID *int64, bggName, bggUrl, bggImageUrl *string) (int64, error) {
	var boardGameID int64
	query := `INSERT INTO boardgames (event_id, name, max_players, bgg_id, bgg_name, bgg_url, bgg_image_url) VALUES (@event_id, @name, @max_players, @bgg_id, @bgg_name, @bgg_url, @bgg_image_url) RETURNING id;`

	if bggImageUrl != nil && *bggImageUrl == "" {
		// Fix for BGG image URLs that contains a filter with mandatory (png)
		tmp := *bggImageUrl
		tmp = strings.ReplaceAll(tmp, "%28", "(")
		tmp = strings.ReplaceAll(tmp, "%29", ")")
		bggImageUrl = &tmp
	}

	if err := d.db.QueryRow(query,
		NamedArgs(map[string]any{
			"event_id":      eventID,
			"name":          name,
			"max_players":   maxPlayers,
			"bgg_id":        bggID,
			"bgg_url":       bggUrl,
			"bgg_name":      bggName,
			"bgg_image_url": bggImageUrl,
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

func (d *Database) UpdateBoardGameBGGInfo(messageID int64, maxPlayers int, bggID *int64, bggName, bggUrl, bggImageUrl *string) error {
	var boardGameID int64

	query := `UPDATE boardgames 
	SET 
	max_players = @max_players,
	bgg_id = @bgg_id,
	bgg_name = @bgg_name,
	bgg_url = @bgg_url,
	bgg_image_url = @bgg_image_url
	WHERE message_id = @message_id RETURNING id;`

	if err := d.db.QueryRow(query,
		NamedArgs(map[string]any{
			"max_players":   maxPlayers,
			"message_id":    messageID,
			"bgg_id":        bggID,
			"bgg_name":      bggName,
			"bgg_url":       bggUrl,
			"bgg_image_url": bggImageUrl,
		})...,
	).Scan(&boardGameID); err != nil {
		return ParseError(err)
	}

	return nil
}

func (d *Database) UpdateBoardGameBGGInfoByID(ID int64, maxPlayers int, bggID *int64, bggName, bggUrl, bggImageUrl *string) error {
	query := `UPDATE boardgames 
	SET 
	max_players = @max_players,
	bgg_id = @bgg_id,
	bgg_name = @bgg_name,
	bgg_url = @bgg_url,
	bgg_image_url = @bgg_image_url
	WHERE id = @id RETURNING id;`

	if err := d.db.QueryRow(query,
		NamedArgs(map[string]any{
			"max_players":   maxPlayers,
			"id":            ID,
			"bgg_id":        bggID,
			"bgg_name":      bggName,
			"bgg_url":       bggUrl,
			"bgg_image_url": bggImageUrl,
		})...,
	).Scan(&ID); err != nil {
		return ParseError(err)
	}

	return nil
}

func (d *Database) DeleteBoardGameByID(ID int64) error {
	query := `DELETE FROM boardgames WHERE id = @id;`

	if _, err := d.db.Exec(query,
		NamedArgs(map[string]any{
			"id": ID,
		})...,
	); err != nil {
		return err
	}

	return nil
}

func (d *Database) HasBoardGameWithMessageID(messageID int64) bool {
	query := `SELECT id FROM boardgames WHERE message_id = @message_id;`

	var id int64
	if err := d.db.QueryRow(query,
		NamedArgs(map[string]any{
			"message_id": messageID,
		})...,
	).Scan(&id); err != nil {
		return false
	}

	return true
}

func (d *Database) InsertParticipant(eventID string, boardgameID, userID int64, userName string) (int64, error) {
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

func (d *Database) RemoveParticipant(eventID string, userID int64) error {
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

func (d *Database) InsertChat(chatID int64, language string) error {
	query := `
		INSERT INTO chats (chat_id, language) 
		VALUES (@chat_id, @language)
		ON CONFLICT (chat_id) 
		DO UPDATE SET language = EXCLUDED.language;
	`

	if _, err := d.db.Exec(query,
		NamedArgs(map[string]any{
			"chat_id":  chatID,
			"language": language,
		})...,
	); err != nil {
		return err
	}

	return nil
}

func (d *Database) GetPreferredLanguage(chatID int64) string {
	query := `SELECT language FROM chats WHERE chat_id = @chat_id;`

	var language string
	if err := d.db.QueryRow(query,
		NamedArgs(map[string]any{
			"chat_id": chatID,
		})...,
	).Scan(&language); err != nil {
		return "en"
	}

	return language
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
