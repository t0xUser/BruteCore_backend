package migrations

const (
	qSearchSettings = "SELECT name FROM sqlite_master WHERE type='table' AND name='SETTINGS';"
	qGetDBVersion   = "SELECT VALUE FROM SETTINGS WHERE KEY = 'DB_VERSION'"
	qCreateSettings = `
		CREATE TABLE SETTINGS(
			key   varchar(60), 
			value varchar(300)
		);
		INSERT INTO SETTINGS VALUES('DB_VERSION', '0');
	`
)

var updateArr map[string]string

func init() {
	//Ключ в мапе это текущая версия конфигурации БД
	//Значение в апдейте это следующая версия которая должна быть после наката версии
	updateArr = map[string]string{
		"0": `

			CREATE TABLE SESSION_COMBOLIST_TMP (
				SESSION_ID INTEGER, 
				TYPE varchar(12),
				"DATA" varchar(300)
			);
			CREATE INDEX SESSION_COMBOLIST_TMP_SESSION_ID_IDX ON SESSION_COMBOLIST_TMP (SESSION_ID);
			CREATE INDEX SESSION_COMBOLIST_TMP_TYPE_IDX ON SESSION_COMBOLIST_TMP (TYPE);


			ALTER TABLE "proxy" DROP COLUMN timeout;
			ALTER TABLE "session" ADD COLUMN timeout INTEGER;
			UPDATE "session" SET timeout = 15000;

			UPDATE SETTINGS SET VALUE = '1' WHERE KEY = 'DB_VERSION';

		`,
		"1": `
		
			CREATE TABLE SESSION_DATA_WEBAPI(
				SESSION_ID INTEGER, 
				ID INTEGER,
				"DATA" VARCHAR(300),
				DATA_STATUS VARCHAR(10)
			);

			CREATE INDEX SESSION_DATA_WEBAPI_SESSION_ID_IDX ON SESSION_DATA_WEBAPI (SESSION_ID);
			CREATE INDEX SESSION_DATA_WEBAPI_ID_IDX ON SESSION_DATA_WEBAPI (ID);
			CREATE INDEX SESSION_DATA_WEBAPI_DATA_STATUS_IDX ON SESSION_DATA_WEBAPI (DATA_STATUS);

			INSERT INTO SESSION_DATA_WEBAPI
			SELECT 
				SD.session_id, 
				ROW_NUMBER() OVER(PARTITION BY SD.session_id ORDER BY SD.session_id) AS rown,
				DLD."data", SD.data_status
			FROM session_data sd 
			INNER JOIN database_link_data dld ON SD.database_id = DLD.database_id AND SD.database_con_id = DLD.con_id;

			WITH L AS (
				SELECT 
					SD.session_id, 
					ROW_NUMBER() OVER(PARTITION BY SD.session_id ORDER BY SD.session_id) AS rown,
					DLD."data", SD.DATABASE_CON_ID
				FROM session_data sd 
				INNER JOIN database_link_data dld ON SD.database_id = DLD.database_id AND SD.database_con_id = DLD.con_id
			)
			UPDATE session_data_log SET ID = (SELECT ROWN FROM L WHERE L.SESSION_ID = session_id AND L.DATABASE_CON_ID = con_id);
			
			DROP TABLE session_data;

			UPDATE SETTINGS SET VALUE = '2' WHERE KEY = 'DB_VERSION';
			
		`,
		"2": `

			INSERT INTO SETTINGS(KEY, VALUE)
			VALUES
				('THREADS', '100'),
				('TIMEOUT', '15000');

			UPDATE SETTINGS SET VALUE = '3' WHERE KEY = 'DB_VERSION';
		`,
		"3": `
			INSERT INTO SESSION_DTL
			SELECT S.ID, 'DATABASE_ID', (SELECT COUNT(1) FROM database_link_data WHERE database_id = S.database_id), '-' 
			FROM SESSION S INNER JOIN MODULE M ON M.ID = S.MODULE_ID WHERE M."type" = 'MT2';

			INSERT INTO SESSION_DTL
			SELECT S.ID, 'USERNAME_ID', (SELECT COUNT(1) FROM database_link_data WHERE database_id = S.username_id), '-' 
			FROM SESSION S INNER JOIN MODULE M ON M.ID = S.MODULE_ID WHERE M."type" = 'MT2';

			INSERT INTO SESSION_DTL
			SELECT S.ID, 'PASSWORD_ID', (SELECT COUNT(1) FROM database_link_data WHERE database_id = S.password_id), '-' 
			FROM SESSION S INNER JOIN MODULE M ON M.ID = S.MODULE_ID WHERE M."type" = 'MT2';


			UPDATE SETTINGS SET VALUE = '4' WHERE KEY = 'DB_VERSION';
		`,
	}
}
