Pod::Spec.new do |spec|
  spec.name         = 'Gpaa'
  spec.version      = '{{.Version}}'
  spec.license      = { :type => 'GNU Lesser General Public License, Version 3.0' }
  spec.homepage     = 'https://github.com/PaloAltoAi/go-PaloAltoAi'
  spec.authors      = { {{range .Contributors}}
		'{{.Name}}' => '{{.Email}}',{{end}}
	}
  spec.summary      = 'iOS PaloAltoAi Client'
  spec.source       = { :git => 'https://github.com/PaloAltoAi/go-PaloAltoAi.git', :commit => '{{.Commit}}' }

	spec.platform = :ios
  spec.ios.deployment_target  = '9.0'
	spec.ios.vendored_frameworks = 'Frameworks/Gpaa.framework'

	spec.prepare_command = <<-CMD
    curl https://gpaastore.blob.core.windows.net/builds/{{.Archive}}.tar.gz | tar -xvz
    mkdir Frameworks
    mv {{.Archive}}/Gpaa.framework Frameworks
    rm -rf {{.Archive}}
  CMD
end
