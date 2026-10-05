CREATE TABLE db_forceproject (
    oid varchar(36) PRIMARY KEY,
    version int NOT NULL,
    data json NOT NULL
);

CREATE TABLE db_forceagentprofile (
    oid varchar(36) PRIMARY KEY,
    version int NOT NULL,
    data json NOT NULL
);
