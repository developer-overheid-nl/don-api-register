
import psycopg2
import json
import requests
from collections import defaultdict

# 1. Verbind met je database
conn = psycopg2.connect(
    dbname="don",
    user="don",
    password="dBfvPRKWC2ELpgUmzGRhaHEdvCJIEzMN69S0KUMTy58WL1Hsm4a5ZzpNWRBA3Bzj",
    host="localhost",
    port=1337
)
cur = conn.cursor()

# 2. Haal alle API metadata (behalve servers/environments) op
cur.execute("""
SELECT
    api.api_id,
    api.api_authentication,
    api.contact_email,
    api.contact_phone,
    api.contact_url,
    api.description,
    api.service_name,
    api.organization_id,
    org.name as organization_name,
    org.contact as org_contact,
    prod_env.specification_url as production_spec_url,
    prod_env.documentation_url as production_doc_url
FROM core_api api
         LEFT JOIN core_organization org ON api.organization_id = org.id
         LEFT JOIN (
    SELECT api_id, specification_url, documentation_url
    FROM core_environment
    WHERE LOWER(name) LIKE '%production%' OR LOWER(name) LIKE '%productie%'
) prod_env ON prod_env.api_id = api.api_id
ORDER BY api.api_id;
""")
apis = cur.fetchall()
api_columns = [desc[0] for desc in cur.description]

# 3. Haal alle environments/servers op
cur.execute("""
SELECT
    api_id,
    name AS description,
    api_url AS uri,
    specification_url AS oasUri,
    documentation_url AS docsUri
FROM core_environment
ORDER BY api_id
""")
env_rows = cur.fetchall()
env_columns = [desc[0] for desc in cur.description]

# 4. Maak mapping: api_id -> list of servers
servers_per_api = defaultdict(list)
for row in env_rows:
    env = dict(zip(env_columns, row))
    servers_per_api[env['api_id']].append({
        "description": env['description'],
        "uri": env['uri']
    })

# 5. Combineer alle info per API
api_list = []
for row in apis:
    record = dict(zip(api_columns, row))
    api_id = record["api_id"]

    org_uri = ""
    contact_json = record.get("org_contact")
    if contact_json:
        # AL dict? (uit psycopg2 bij JSON/JSONB columns vaak het geval!)
        if isinstance(contact_json, dict):
            internetadressen = contact_json.get("internetadressen", [])
            print(internetadressen)

        else:
            try:
                contact_json = json.loads(contact_json)
                internetadressen = contact_json.get("internetadressen", [])
            except Exception as e:
                print(f"[DEBUG] Failed to parse contact JSON for {record.get('organization_name')}: {e}")
                internetadressen = []
        if internetadressen and isinstance(internetadressen, list):
            org_uri = internetadressen[0].get("url", "")
    api_obj = {
        "title": record["service_name"],
        "description": record["description"],
        "oasUri": record["production_spec_url"] or "",
        "docsUri": record["production_doc_url"],
        "auth": record["api_authentication"],
        "contact_email": record["contact_email"],
        "contact_url": record["contact_url"],
        "organisation": {
            "label": record["organization_name"],
            "uri": org_uri or "",
        },
        "servers": servers_per_api.get(api_id, [])
    }
    api_list.append(api_obj)

print(f"Totaal te posten: {len(api_list)} API's")

# 6. Post elk API-object naar je nieuwe API-register (Go app)
endpoint = "http://localhost:1338/apis/v1/apis"  # Pas aan indien nodig
headers = {"Content-Type": "application/json"}

for api in api_list:
    resp = requests.post(endpoint, headers=headers, json=api)
    # print(api.get("title"), resp.status_code, resp.text)
    # if resp.status_code == 409:
    #     print(f"⚠️ API bestaat al: {api.get('title')}")
    # elif resp.status_code >= 400:
    #     print(f"❌ Fout bij API: {api.get('title')}: {resp.text}")

# 7. Sluit connectie
cur.close()
conn.close()
