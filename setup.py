from setuptools import setup, find_packages

setup(name='hdhr-legacy-proxy',
      version='0.0.1',
      description="Proxy for legacy HDHomeRun to respond as modern HDHR for applications like Plex",
      long_description="",
      classifiers=['Development Status :: 4 - Beta',
                   'Natural Language :: English',
                   'Topic :: Multimedia :: Video :: Capture'],
      keywords='tv television tuner tvtuner hdhomerun',
      author='Jason Kincl',
      author_email='jason@kincl.dev',
      url='https://github.com/kincl/hdhr-legacy-proxy',
      license='MIT',
      packages=['hdhr_legacy_proxy'],
      include_package_data=True,
      zip_safe=True,
      setup_requires=['wheel'],
      install_requires=['setuptools']
)
