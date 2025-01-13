CREATE TABLE contestant (
    name TEXT,
    video_id TEXT PRIMARY KEY
);

CREATE TABLE statistic (
    id SERIAL PRIMARY KEY,
    video_id text,
    FOREIGN KEY (video_id) REFERENCES contestant(video_id),
    view_count INTEGER,
    updated TIMESTAMP
);

COPY contestant (name, video_id)
FROM '/docker-entrypoint-initdb.d/contestant.csv'
DELIMITER ','
CSV HEADER;
