
BEGIN TRANSACTION; 

	CREATE TABLE users(
		username TEXT UNIQUE, 
	 	password TEXT		

	);

CREATE TABLE pages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    title TEXT NOT NULL UNIQUE,
    body BLOB NOT NULL,
    entry_date DATETIME, 
    entry_type TEXT NOT NULL DEFAULT 'theme', 
created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

insert into pages (title, body, entry_type)
values ('test', 'hello world', 'theme'),('hello world', 'this is a test of hello world', 'theme');

insert into users (username, password)
values ('user1', '123'), ('user2', '123'), ('user3', '123');

COMMIT;
