import os
import yaml
import requests

def find_all_specification_metadata(folder_path: str) -> list[dict]:
    spec_list = []

    for root, _, files in os.walk(folder_path):
        for file in files:
            if file.endswith((".yaml", ".yml")):
                full_path = os.path.join(root, file)
                try:
                    with open(full_path, "r", encoding="utf-8") as f:
                        content = yaml.safe_load(f)

                        def collect_spec_data(data):
                            if isinstance(data, dict):
                                for env in data.get("environments", []):
                                    if "specification_url" in env:
                                        entry = {
                                            "oasUri": env["specification_url"],
                                            "docsUri": env.get("documentation_url"),
                                            "title": data.get("service_name"),
                                            "description": data.get("description") if isinstance(data.get("description"), str) else None,
                                            "auth": data.get("api_authentication"),
                                            "type": data.get("api_type"),
                                            "organisation": {
                                                "label": data.get("organization", {}).get("name"),
                                                "uri": f"https://organisatie.overheid.nl/{data.get('organization', {}).get('ooid')}"
                                                if data.get("organization", {}).get("ooid") else None
                                            }
                                        }
                                        spec_list.append(entry)

                        collect_spec_data(content)

                except Exception as e:
                    print(f"❌ Fout bij verwerken van {full_path}: {e}")

    return spec_list

def post_and_patch_api_specs(spec_data_list: list[dict], endpointpost: str, endpointpatch: str):
    headers = {"Content-Type": "application/json"}

    for spec in spec_data_list:
        try:
            # 1. POST alleen de oasUri
            post_resp = requests.post(endpointpost, headers=headers, json={"oasUrl": spec["oasUri"]})
            if post_resp.status_code == 201:
                try:
                    created_api = post_resp.json()
                    api_id = created_api.get("id") or created_api.get("Id")
                except Exception as e:
                    print(f"⚠️ JSON decode fout bij POST response: {e}")
                    continue
            elif post_resp.status_code == 409:
                print(f"⚠️ Bestond al: {spec['oasUri']}")
                continue
            else:
                print(f"❌ POST fout {spec['oasUri']}: {post_resp.status_code} {post_resp.text}")
                continue

            # 2. PUT met gecombineerde data
            if api_id:
                patch_url = f"{endpointpatch}{api_id}"

                combined = created_api.copy()
                if spec.get("docsUri"):
                    combined["docsUri"] = spec["docsUri"]
                if spec.get("type"):
                    combined["type"] = spec["type"]
                put_resp = requests.put(patch_url, headers=headers, json=combined)
                if put_resp.status_code != 200:
                    print(f"PUT fout {patch_url}: {put_resp.status_code} {put_resp.text}")
            else:
                print(f"Geen ID gevonden in POST-response voor {spec['oasUri']}")

        except Exception as e:
            print(f"Fout bij verwerken van {spec['oasUri']}: {e}")


if __name__ == "__main__":
    folder = "/Users/matthijshovestad/workspace/geonovum/don-content/content/api"
    endpointPost = "http://localhost:1338/apis/v1/oas"
    endpointPatch = "http://localhost:1338/apis/v1/api/"

    specs = find_all_specification_metadata(folder)
    print(f"{len(specs)} specificaties gevonden.")
    post_and_patch_api_specs(specs, endpointPost, endpointPatch)
