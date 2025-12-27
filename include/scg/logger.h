#pragma once

#include <string>
#include <functional>

namespace scg {
namespace log {

// Simple logger interface
class Logger {
public:
	virtual ~Logger() = default;
	virtual void debug(const std::string& msg) = 0;
	virtual void info(const std::string& msg) = 0;
	virtual void warn(const std::string& msg) = 0;
	virtual void error(const std::string& msg) = 0;
};

enum class LogLevel {
	DEBUG,
	INFO,
	WARN,
	ERROR,
	NONE
};

struct LoggingConfig {
	LogLevel level = LogLevel::INFO;
	std::function<void(std::string)> debugLogger;
	std::function<void(std::string)> infoLogger;
	std::function<void(std::string)> warnLogger;
	std::function<void(std::string)> errorLogger;
};

}
}
