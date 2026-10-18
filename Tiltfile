# Inner development loop (plan.md §14.3).

registry = 'localhost:5050'

# Five services share the default runtime stage; scan-worker needs the image
# with the git binary.
plain_services = ['api', 'discovery-worker', 'enricher', 'notifier', 'analytics']

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

for name in plain_services:
    docker_build(
        registry + '/' + name,
        context='.',
        dockerfile='build/Dockerfile',
        target='runtime',
        build_args={'SERVICE': name},
        only=service_deps(name),
    )

docker_build(
    registry + '/scan-worker',
    context='.',
    dockerfile='build/Dockerfile',
    target='runtime-git',
    build_args={'SERVICE': 'scan-worker'},
    only=service_deps('scan-worker'),
)

k8s_yaml(helm(
    'deploy/chart',
    name='gitscanner',
    namespace='gitscanner',
    values=['deploy/values/dev.yaml'],
))

k8s_resource('api', port_forwards=['8080:8080'])
for name in ['discovery-worker', 'scan-worker', 'enricher', 'notifier', 'analytics']:
    k8s_resource(name)
