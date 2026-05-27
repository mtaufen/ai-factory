import os
import time
import yaml
import logging
from kubernetes import client, config
from kubernetes.client.rest import ApiException

logging.basicConfig(level=logging.INFO, format='%(asctime)s %(message)s')

AGENTS_DIR = os.environ.get("AGENTS_DIR", "/app/.agents")
NAMESPACE = os.environ.get("NAMESPACE", "default")

def get_namespace():
    ns_path = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"
    if os.path.exists(ns_path):
        with open(ns_path, 'r') as f:
            return f.read().strip()
    return NAMESPACE

def get_agent_configs():
    agents = {}
    if not os.path.exists(AGENTS_DIR):
        return agents
    for entry in os.listdir(AGENTS_DIR):
        agent_dir = os.path.join(AGENTS_DIR, entry)
        if os.path.isdir(agent_dir):
            agent_md_path = os.path.join(agent_dir, "agent.md")
            if os.path.exists(agent_md_path):
                try:
                    with open(agent_md_path, 'r') as f:
                        content = f.read()
                        if content.startswith("---"):
                            parts = content.split("---", 2)
                            if len(parts) >= 3:
                                frontmatter = yaml.safe_load(parts[1])
                                agents[entry] = frontmatter or {}
                except Exception as e:
                    logging.error(f"Error parsing {agent_md_path}: {e}")
    return agents

def get_sandbox_manifest(name, metadata):
    schedule = metadata.get("schedule", "")
    return {
        "apiVersion": "agents.x-k8s.io/v1beta1",
        "kind": "Sandbox",
        "metadata": {
            "name": name,
            "annotations": {
                "agents.x-k8s.io/schedule": schedule
            }
        },
        "spec": {
            "podTemplate": {
                "spec": {
                    "containers": [
                        {
                            "name": "agent",
                            "image": "busybox",
                            "command": ["sleep", "3600"]
                        }
                    ]
                }
            }
        }
    }

def reconcile(api):
    ns = get_namespace()
    
    group = "agents.x-k8s.io"
    version = "v1beta1"
    plural = "sandboxes"
    
    # 1. List current sandboxes
    try:
        sandboxes = api.list_namespaced_custom_object(group, version, ns, plural)
        existing = {item['metadata']['name']: item for item in sandboxes.get('items', [])}
    except ApiException as e:
        if e.status == 404:
            logging.error("CRD not found. Skipping.")
            return
        logging.error(f"Error listing sandboxes: {e}")
        existing = {}

    # 2. Get desired agents from local repo
    desired = get_agent_configs()

    # 3. Create or update
    for name, metadata in desired.items():
        schedule = metadata.get("schedule", "")
        if name not in existing:
            manifest = get_sandbox_manifest(name, metadata)
            try:
                api.create_namespaced_custom_object(group, version, ns, plural, manifest)
                logging.info(f"Created Sandbox for {name}")
            except ApiException as e:
                logging.error(f"Failed to create Sandbox for {name}: {e}")
        else:
            # Check drift
            current_sandbox = existing[name]
            current_schedule = current_sandbox.get("metadata", {}).get("annotations", {}).get("agents.x-k8s.io/schedule", "")
            if current_schedule != schedule:
                try:
                    # Patch
                    api.patch_namespaced_custom_object(group, version, ns, plural, name, {"metadata": {"annotations": {"agents.x-k8s.io/schedule": schedule}}})
                    logging.info(f"Reconciled drift for {name}")
                except ApiException as e:
                    logging.error(f"Failed to update Sandbox for {name}: {e}")

    # 4. Delete removed
    for name in existing.keys():
        if name not in desired:
            try:
                api.delete_namespaced_custom_object(group, version, ns, plural, name)
                logging.info(f"Deleted Sandbox for {name}")
            except ApiException as e:
                logging.error(f"Failed to delete Sandbox for {name}: {e}")

if __name__ == "__main__":
    try:
        config.load_incluster_config()
    except config.ConfigException:
        try:
            config.load_kube_config()
        except config.ConfigException:
            logging.error("Could not load K8s config. Exiting.")
            exit(1)

    api = client.CustomObjectsApi()
    
    while True:
        reconcile(api)
        time.sleep(10)
