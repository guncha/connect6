module.exports = (grunt) ->
  grunt.initConfig
    clean: ['dist/', 'build/']

    coffee:
      compile:
        expand: true
        cwd: 'js/'
        src: ['*.coffee']
        dest: 'build/'
        ext: '.js'

    browserify:
      'dist/connect6.js': ['build/main.js']

    watch:
      files: 'js/*'
      tasks: 'default'

  grunt.loadNpmTasks 'grunt-contrib-clean'
  grunt.loadNpmTasks 'grunt-browserify'
  grunt.loadNpmTasks('grunt-contrib-coffee');
  grunt.loadNpmTasks('grunt-contrib-watch');

  grunt.registerTask 'default', ['clean', 'coffee', 'browserify']
