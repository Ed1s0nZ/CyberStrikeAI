from minidump.streams.SystemInfoStream import PROCESSOR_ARCHITECTURE


PEB_OFFSETS = [
	{  # x86 offsets
		# _TEB offsets
		"peb": 0x30,
		# _PEB offsets
		"being_debugged": 0x2,
		"image_base_address": 0x8,
		"process_parameters": 0x10,
		# _RTL_USER_PROCESS_PARAMETERS offsets
		"image_path": 0x38,
		"command_line": 0x40,
		"window_title": 0x70,
		"dll_path": 0x30,
		"current_directory": 0x24,
		"standard_input": 0x18,
		"standard_output": 0x1C,
		"standard_error": 0x20,
		"environment_variables": 0x48,
		# _UNICODE_STRING offsets
		"buffer": 0x4,
	},
	{  # x64 offsets
		# _TEB offsets
		"peb": 0x60,
		# _PEB offsets
		"being_debugged": 0x2,
		"image_base_address": 0x10,
		"process_parameters": 0x20,
		# _RTL_USER_PROCESS_PARAMETERS offsets
		"image_path": 0x60,
		"command_line": 0x70,
		"window_title": 0xB0,
		"dll_path": 0x50,
		"current_directory": 0x38,
		"standard_input": 0x20,
		"standard_output": 0x28,
		"standard_error": 0x30,
		"environment_variables": 0x80,
		# _UNICODE_STRING offsets
		"buffer": 0x8,
	},
]

class PEB:
	def __init__(self):
		self.address = None
		self.is_x64 = None
		self.ptr_size = None
		self.being_debugged = None
		self.image_base_address = None
		self.process_parameters = None
		self.image_path = None
		self.command_line = None
		self.window_title = None
		self.dll_path = None
		self.current_directory = None
		self.standard_input = None
		self.standard_output = None
		self.standard_error = None
		self.environment_variables = []
	
	def read_unicode_string_property(self, reader, addr):
		reader.move(addr)
		string_length = int.from_bytes(reader.read(2), "little")
		if not string_length:
			return ""
		reader.move(addr + PEB_OFFSETS[self.is_x64]["buffer"])
		buff_va = int.from_bytes(reader.read(self.ptr_size), "little")
		reader.move(buff_va)
		return reader.read(string_length).decode("utf-16")
	
	@staticmethod
	def from_minidump(minidumpfile):
		reader = minidumpfile.get_reader()
		buff_reader = reader.get_buffered_reader()

		peb = PEB()
		peb.is_x64 = not(reader.sysinfo.ProcessorArchitecture == PROCESSOR_ARCHITECTURE.INTEL) #dunno if this is the best way...
		peb.ptr_size = 8 if peb.is_x64 else 4
		offset_index = int(peb.is_x64)		

		buff_reader.move(minidumpfile.threads.threads[0].Teb + PEB_OFFSETS[offset_index]["peb"])

		peb.address = int.from_bytes(buff_reader.read(peb.ptr_size), "little")

		buff_reader.move(peb.address + PEB_OFFSETS[offset_index]["being_debugged"])
		peb.being_debugged = int.from_bytes(buff_reader.read(1), "little")

		buff_reader.move(peb.address + PEB_OFFSETS[offset_index]["image_base_address"])
		peb.image_base_address = int.from_bytes(buff_reader.read(peb.ptr_size), "little")

		buff_reader.move(peb.address + PEB_OFFSETS[offset_index]["process_parameters"])
		process_parameters = int.from_bytes(buff_reader.read(peb.ptr_size), "little")

		peb.image_path = peb.read_unicode_string_property(
			buff_reader, process_parameters + PEB_OFFSETS[offset_index]["image_path"]
		)

		peb.command_line = peb.read_unicode_string_property(
			buff_reader, process_parameters + PEB_OFFSETS[offset_index]["command_line"]
		)

		peb.window_title = peb.read_unicode_string_property(
			buff_reader, process_parameters + PEB_OFFSETS[offset_index]["window_title"]
		)

		peb.dll_path = peb.read_unicode_string_property(buff_reader, process_parameters + PEB_OFFSETS[offset_index]["dll_path"])

		peb.current_directory = peb.read_unicode_string_property(
			buff_reader, process_parameters + PEB_OFFSETS[offset_index]["current_directory"]
		)

		buff_reader.move(process_parameters + PEB_OFFSETS[offset_index]["standard_input"])
		peb.standard_input = int.from_bytes(buff_reader.read(peb.ptr_size), "little")

		buff_reader.move(process_parameters + PEB_OFFSETS[offset_index]["standard_output"])
		peb.standard_output = int.from_bytes(buff_reader.read(peb.ptr_size), "little")

		buff_reader.move(process_parameters + PEB_OFFSETS[offset_index]["standard_error"])
		peb.standard_error = int.from_bytes(buff_reader.read(peb.ptr_size), "little")

		# Parse Environment Variables from PEB
		buff_reader.move(process_parameters + PEB_OFFSETS[offset_index]["environment_variables"])
		environment_va = int.from_bytes(buff_reader.read(peb.ptr_size), "little")
		buff_reader.move(environment_va)

		env_buffer = buff_reader.read(buff_reader.current_segment.end_address - buff_reader.current_position)
		while (env_len := env_buffer.find(b"\x00\x00")) and (env_len != -1):
			decoded_env = (env_buffer[:env_len] + b"\x00").decode("utf-16")
			name = decoded_env
			value = ""
			if decoded_env.find("=") != -1:
				name, value = decoded_env.split("=", 1)
			peb.environment_variables.append({"name": name, "value": value})
			environment_va += (len(decoded_env) + 1) * 2
			buff_reader.move(environment_va)
			env_buffer = buff_reader.read(buff_reader.current_segment.end_address - buff_reader.current_position)

		return peb

	def __str__(self):
		envs = "\n\t".join([f"{env['name']}={env['value']}" for env in self.environment_variables])
		return f"""
PEB ADDR: {hex(self.address)}
BeingDebugged: {self.being_debugged}
ImageBaseAddress: {hex(self.image_base_address)}
ProcessParameters: {self.process_parameters}
ImagePath: {self.image_path}
CommandLine: {self.command_line}
WindowTitle: {self.window_title}
DllPath: {self.dll_path}
CurrentDirectory: {self.current_directory}
StandardInput: {self.standard_input}
StandardOutput: {self.standard_output}
StandardError: {self.standard_error}
EnvironmentVariables: 
	{envs}
"""
	
	def __repr__(self):
		return self.__str__()
	

