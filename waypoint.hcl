# The name of your project. A project typically maps 1:1 to a VCS repository.
# This name must be unique for your Waypoint server. If you're running in
# local mode, this must be unique to your machine.
project = "prometheus-retention"

# Labels can be specified for organizational purposes.
# labels = { "foo" = "bar" }

# An application to deploy.
app "retention" {
    # Build specifies how an application should be deployed. In this case,
    # we'll build using a Dockerfile and keeping it in a local registry.
    build {
        use "docker" {
          buildkit = true
        }
                
        registry {
          use "docker" {
            image = "mystik738/prometheus-retention"
            tag   = "latest"
            local = true
          }
        }
    }

    # Deploy to Docker
    deploy {
        use "kubernetes" {
        }
    }
}
