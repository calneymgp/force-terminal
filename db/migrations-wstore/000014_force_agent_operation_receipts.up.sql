CREATE TABLE force_agent_request_ledger (
    instance_id varchar(36) NOT NULL,
    request_key varchar(36) NOT NULL,
    intent text NOT NULL,
    generation integer NOT NULL,
    kind text NOT NULL,
    accepted_at integer NOT NULL,
    PRIMARY KEY (instance_id, request_key)
);

INSERT INTO force_agent_request_ledger (instance_id, request_key, intent, generation, kind, accepted_at)
SELECT oid,
       json_extract(data, '$.operationrequestkey'),
       json_extract(data, '$.operationintent'),
       COALESCE(json_extract(data, '$.generation'), 0),
       'operation',
       COALESCE(json_extract(data, '$.updatedat'), 0)
FROM db_forceagentinstance
WHERE json_extract(data, '$.operationrequestkey') IS NOT NULL
  AND json_extract(data, '$.operationrequestkey') != ''
  AND json_extract(data, '$.operationintent') IS NOT NULL
  AND json_extract(data, '$.operationintent') != '';
