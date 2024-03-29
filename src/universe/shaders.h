#pragma once

#include <glpp/gl.h>
#include <universe/shaders/shader_sources.h>

struct Shaders : public gl::Shaders {
	typedef ShaderSources Src;

	struct BasicProgram : gl::Program {
		in_vec4 position = {"position"};
		in_vec4 color = {"color"};

		BasicProgram()
				: Program("BasicProgram", Src::basic_v, Src::basic_f) { }
	};
	const BasicProgram basic_program;

	struct SolidProgram : gl::Program {
		uniform_mat4 projection = {"projection"};
		uniform_mat4 model = {"model"};
		uniform_vec4 color = {"color"};
		in_vec3 position = {"position"};
		in_vec3 normal = {"normal"};

		SolidProgram()
				: Program("SolidProgram", Src::solid_v, Src::solid_f) { }
	};
	const SolidProgram solid_program;
};