package serverconfig

type Config struct {
	MySQL       MySQLConfig       `yaml:"mysql" mapstructure:"mysql"`
	HTTPServer  HTTPServerConfig  `yaml:"httpserver" mapstructure:"httpserver"`
	GateServer  GateServerConfig  `yaml:"gateserver" mapstructure:"gateserver"`
	SLGServer   SLGServerConfig   `yaml:"slgserver" mapstructure:"slgserver"`
	ChatServer  ChatServerConfig  `yaml:"chatserver" mapstructure:"chatserver"`
	LoginServer LoginServerConfig `yaml:"loginserver" mapstructure:"loginserver"`
	Xorm        XormConfig        `yaml:"xorm" mapstructure:"xorm"`
	Log         LogConfig         `yaml:"log" mapstructure:"log"`
	Logic       LogicConfig       `yaml:"logic" mapstructure:"logic"`
}

type MySQLConfig struct {
	Host     string `yaml:"host" mapstructure:"host"`
	Port     int    `yaml:"port" mapstructure:"port"`
	User     string `yaml:"user" mapstructure:"user"`
	Password string `yaml:"password" mapstructure:"password"`
	DBName   string `yaml:"dbname" mapstructure:"dbname"`
	Charset  string `yaml:"charset" mapstructure:"charset"`
	MaxIdle  int    `yaml:"max_idle" mapstructure:"max_idle"`
	MaxConn  int    `yaml:"max_conn" mapstructure:"max_conn"`
}

type HTTPServerConfig struct {
	Host string `yaml:"host" mapstructure:"host"`
	Port int    `yaml:"port" mapstructure:"port"`
}

type GateServerConfig struct {
	Host       string `yaml:"host" mapstructure:"host"`
	Port       int    `yaml:"port" mapstructure:"port"`
	NeedSecret bool   `yaml:"need_secret" mapstructure:"need_secret"`
	SLGProxy   string `yaml:"slg_proxy" mapstructure:"slg_proxy"`
	ChatProxy  string `yaml:"chat_proxy" mapstructure:"chat_proxy"`
	LoginProxy string `yaml:"login_proxy" mapstructure:"login_proxy"`
}

type SLGServerConfig struct {
	Host       string `yaml:"host" mapstructure:"host"`
	Port       int    `yaml:"port" mapstructure:"port"`
	NeedSecret bool   `yaml:"need_secret" mapstructure:"need_secret"`
	IsDev      bool   `yaml:"is_dev" mapstructure:"is_dev"`
}

type ChatServerConfig struct {
	Host       string `yaml:"host" mapstructure:"host"`
	Port       int    `yaml:"port" mapstructure:"port"`
	NeedSecret bool   `yaml:"need_secret" mapstructure:"need_secret"`
}

type LoginServerConfig struct {
	Host       string `yaml:"host" mapstructure:"host"`
	Port       int    `yaml:"port" mapstructure:"port"`
	NeedSecret bool   `yaml:"need_secret" mapstructure:"need_secret"`
}

type XormConfig struct {
	ShowSQL bool   `yaml:"show_sql" mapstructure:"show_sql"`
	LogFile string `yaml:"log_file" mapstructure:"log_file"`
}

type LogConfig struct {
	FileDir    string `yaml:"file_dir" mapstructure:"file_dir"`
	MaxSize    int    `yaml:"max_size" mapstructure:"max_size"` // MB
	MaxBackups int    `yaml:"max_backups" mapstructure:"max_backups"`
	MaxAge     int    `yaml:"max_age" mapstructure:"max_age"` // days
	Compress   bool   `yaml:"compress" mapstructure:"compress"`
	Level      string `yaml:"level" mapstructure:"level"` // debug/info/warn/error...
	Dev        bool   `yaml:"dev" mapstructure:"dev"`
}

type LogicConfig struct {
	MapData  string `yaml:"map_data" mapstructure:"map_data"`
	JSONData string `yaml:"json_data" mapstructure:"json_data"`
	ServerID int    `yaml:"server_id" mapstructure:"server_id"`
}
