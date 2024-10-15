migrate-up: 
	goose -dir ./internal/migrations postgres "user=user dbname=geo_match_db password=password sslmode=disable host=localhost" up
migrate-down: 
	goose -dir ./internal/migrations postgres "user=user dbname=geo_match_db password=password sslmode=disable host=localhost" down