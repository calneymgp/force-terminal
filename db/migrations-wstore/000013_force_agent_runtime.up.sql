CREATE TABLE db_forceagentinstance (
    oid varchar(36) PRIMARY KEY,
    version int NOT NULL,
    data json NOT NULL
);

CREATE INDEX idx_forceagentinstance_project ON db_forceagentinstance (json_extract(data, '$.projectid'));
CREATE UNIQUE INDEX idx_forceagentinstance_block ON db_forceagentinstance (json_extract(data, '$.blockid'));

CREATE TABLE force_agent_writer_lease (
    destination_key text PRIMARY KEY,
    host_identity text NOT NULL,
    canonical_root text NOT NULL,
    instance_id varchar(36) NOT NULL,
    generation integer NOT NULL
);

CREATE INDEX idx_force_agent_writer_host ON force_agent_writer_lease (host_identity);
