terraform { 
  cloud {     
    organization = "terraform-tom" 
    workspaces { 
      name = "local-runtask-test" 
    } 
  } 
}

resource "random_pet" "main" {
  keepers = {
    always = timestamp()
  }
  length = 4
}