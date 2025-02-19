CREATE TABLE contestant (
    id SERIAL PRIMARY KEY,
    name TEXT,
    video_id TEXT UNIQUE
    event TEXT CHECK (event IN ('umk', 'eurovision')) NOT NULL DEFAULT 'umk'
);

CREATE TABLE statistic (
    id SERIAL PRIMARY KEY,
    video_id text,
    FOREIGN KEY (video_id) REFERENCES contestant(video_id),
    view_count INTEGER,
    updated TIMESTAMP
);

COPY contestant (name, video_id, event)
FROM '/docker-entrypoint-initdb.d/contestant.csv'
DELIMITER ','
CSV HEADER;
