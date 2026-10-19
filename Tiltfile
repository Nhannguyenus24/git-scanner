# Inner development loop (plan.md §14.3).

registry = 'localhost:5050'

# Every directory under cmd/ is a service, same rule as the Makefile, so adding
# a service never means editing this file.
services = [
    os.path.basename(os.path.dirname(f))
    for f in listdir('cmd', recursive=True)
    if os.path.basename(f) == 'main.go'
]

# Services that shell out to git and need the runtime-git image (plan.md §13.3).
git_services = ['scan-worker']

def service_deps(name):
    # The `only` list is what makes a monorepo tolerable locally: editing
    # internal/notify must not rebuild six images. Every service lists libs, so
    # a change there does rebuild everything -- that is correct, and it is the
    # cost plan.md §3.4 warns about.
    return [
        'go.mod',
        'go.sum',
        'cmd/' + name,
        'libs',
        'internal',
    ]

for name in services:
    docker_build(
        registry + '/' + name,
        context='.',
        dockerfile='build/Dockerfile',
        target='runtime-git' if name in git_services else 'runtime',
        build_args={'SERVICE': name},
        only=service_deps(name),
    )

k8s_yaml(helm(
    'deploy/chart',
    name='gitscanner',
    namespace='gitscanner',
    values=['deploy/values/dev.yaml'],
))

for name in services:
    if name == 'api':
        k8s_resource(name, port_forwards=['8080:8080'])
    else:
        k8s_resource(name)
