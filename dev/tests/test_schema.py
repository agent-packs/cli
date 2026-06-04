import copy
import json
import re
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
SCHEMA_PATH = ROOT / "dev" / "schemas" / "agent-pack.schema.json"
REGISTRY_PATH = ROOT / "dev" / "registry" / "packs"
EXAMPLES_PATH = ROOT / "dev" / "schemas" / "examples"


def load_schema():
    with SCHEMA_PATH.open(encoding="utf-8") as handle:
        return json.load(handle)


def load_pack(path):
    with path.open(encoding="utf-8") as handle:
        return json.load(handle)


def validate_pack(pack, schema):
    errors = []

    if not isinstance(pack, dict):
        return ["pack must be an object"]

    for field in schema["required"]:
        if field not in pack:
            errors.append(f"missing required field: {field}")

    properties = schema["properties"]

    for field, definition in properties.items():
        if field not in pack:
            continue

        expected_type = definition.get("type")
        if expected_type == "string" and not isinstance(pack[field], str):
            errors.append(f"{field} must be a string")
        if expected_type == "array" and not isinstance(pack[field], list):
            errors.append(f"{field} must be an array")

    id_pattern = properties["id"]["pattern"]
    if isinstance(pack.get("id"), str) and not re.fullmatch(id_pattern, pack["id"]):
        errors.append("id does not match schema pattern")

    if "tags" in pack and isinstance(pack["tags"], list):
        for index, tag in enumerate(pack["tags"]):
            if not isinstance(tag, str):
                errors.append(f"tags[{index}] must be a string")

    capabilities = pack.get("capabilities")
    capability_schema = properties["capabilities"]["items"]
    valid_capability_types = set(capability_schema["properties"]["type"]["enum"])

    if isinstance(capabilities, list):
        for index, capability in enumerate(capabilities):
            if not isinstance(capability, dict):
                errors.append(f"capabilities[{index}] must be an object")
                continue

            for field in capability_schema["required"]:
                if field not in capability:
                    errors.append(f"capabilities[{index}] missing required field: {field}")

            for field in ("type", "name", "source"):
                if field in capability and not isinstance(capability[field], str):
                    errors.append(f"capabilities[{index}].{field} must be a string")

            capability_type = capability.get("type")
            if isinstance(capability_type, str) and capability_type not in valid_capability_types:
                errors.append(f"capabilities[{index}].type is not allowed")

    return errors


def valid_pack():
    return {
        "id": "example-pack",
        "name": "Example Pack",
        "version": "0.1.0",
        "description": "A valid pack for tests.",
        "license": "Apache-2.0",
        "tags": ["example"],
        "capabilities": [
            {
                "type": "skill",
                "name": "Example skill",
                "source": "https://example.com/skill",
            }
        ],
    }


class AgentPackSchemaTest(unittest.TestCase):
    def setUp(self):
        self.schema = load_schema()

    def assert_valid(self, pack):
        self.assertEqual(validate_pack(pack, self.schema), [])

    def assert_invalid(self, pack, expected_error):
        errors = validate_pack(pack, self.schema)
        self.assertIn(expected_error, errors)

    def test_registry_packs_match_schema(self):
        pack_paths = sorted(REGISTRY_PATH.glob("*.json"))
        self.assertGreater(len(pack_paths), 0)

        for path in pack_paths:
            with self.subTest(path=path.name):
                self.assert_valid(load_pack(path))

    def test_example_packs_match_schema(self):
        example_paths = sorted(EXAMPLES_PATH.glob("*.json"))
        self.assertGreater(len(example_paths), 0)

        for path in example_paths:
            with self.subTest(path=path.name):
                self.assert_valid(load_pack(path))

    def test_valid_pack_matches_schema(self):
        self.assert_valid(valid_pack())

    def test_requires_top_level_fields(self):
        for field in self.schema["required"]:
            with self.subTest(field=field):
                pack = valid_pack()
                del pack[field]
                self.assert_invalid(pack, f"missing required field: {field}")

    def test_rejects_invalid_id_format(self):
        for bad_id in ["Frontend", "-frontend", "frontend-", "front_end"]:
            with self.subTest(bad_id=bad_id):
                pack = valid_pack()
                pack["id"] = bad_id
                self.assert_invalid(pack, "id does not match schema pattern")

    def test_rejects_unknown_capability_type(self):
        pack = valid_pack()
        pack["capabilities"][0]["type"] = "unknown"
        self.assert_invalid(pack, "capabilities[0].type is not allowed")

    def test_requires_capability_fields(self):
        for field in ["type", "name", "source"]:
            with self.subTest(field=field):
                pack = copy.deepcopy(valid_pack())
                del pack["capabilities"][0][field]
                self.assert_invalid(pack, f"capabilities[0] missing required field: {field}")

    def test_rejects_non_string_tags(self):
        pack = valid_pack()
        pack["tags"] = ["example", 123]
        self.assert_invalid(pack, "tags[1] must be a string")


if __name__ == "__main__":
    unittest.main()
