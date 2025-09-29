locals {
  organization_name = "terraform-tom"
  workspace_name    = "local-runtask-test"
  run_task_name    = "local-runtask-test"
  run_task_url     = "${chomp(file("../../bin/tunnel/url.txt"))}/runtask"
}

resource "tfe_organization_run_task" "main" {
  organization = local.organization_name
  name         = local.run_task_name
  url          = local.run_task_url
  enabled      = true
  description  = "A run task for testing purposes that runs locally on my machine."
}

resource "tfe_workspace" "main" {
  organization = local.organization_name
  name         = local.workspace_name
}

resource "tfe_workspace_run_task" "main" {
  workspace_id      = tfe_workspace.main.id
  task_id           = tfe_organization_run_task.main.id
  enforcement_level = "advisory"
  stages            = ["pre_plan", "post_plan", "pre_apply", "post_apply"]
}
