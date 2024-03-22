package engine

const (
	QUpdateProtocolSessionDataGood = `
		UPDATE SESSION_DATA_PROTOCOL SET DATA_STATUS = '%s' WHERE SESSION_ID = %d AND ID = %d;
		UPDATE SESSION_DATA_PROTOCOL SET DATA_STATUS = 'RT2' WHERE SESSION_ID = %d AND H_DATA = '%s' AND DATA_STATUS IS NULL;
	`

	QUpdateProtocolSessionDataAny = `
		UPDATE SESSION_DATA_PROTOCOL SET DATA_STATUS = '%s' WHERE SESSION_ID = %d AND ID = %d;
	`

	QUpdateSessionWebApi = `
		UPDATE SESSION_DATA_WEBAPI SET DATA_STATUS = '%s' WHERE SESSION_ID = %d AND ID = %d;
	`

	QInsertSessionDataLog = `
		INSERT INTO SESSION_DATA_LOG(SESSION_ID, ID, LOG)
		VALUES(%d, %d, '%s');
	`

	QUpdateErrorStat = `
		UPDATE SESSION_DTL SET
			VALUE = VALUE + 1
		WHERE SESSION_ID = %d AND KEY = 'R1E';
	`

	QFinishSession = `
		UPDATE SESSION SET
			STATUS = "ST6",
			FINISH_TIME = CURRENT_TIMESTAMP
		WHERE ID = %d;
	`

	QGetComboListBatch = `
		SELECT 
			id id, data data, '' login, '' password 
		FROM SESSION_DATA_WEBAPI
		WHERE SESSION_ID = $1 AND ID > $2 LIMIT $3
	`

	QGetComboListProtocolBatch = `
		SELECT 
			id id, h_data data, u_data login, p_data password, data_con_id con_id
		FROM session_data_protocol
		WHERE SESSION_ID = $1 AND ID > $2 LIMIT $3
	`

	QGetComboListBatchStart = `
		SELECT id id, data data, '' login, '' password
		FROM SESSION_DATA_WEBAPI
		WHERE SESSION_ID = $1 AND DATA_STATUS IS NULL
		LIMIT $2
	`

	QGetComboListProtocolBatchStart = `
		SELECT 
			id id, h_data data, u_data login, p_data password 
		FROM SESSION_DATA_PROTOCOL SDP
		WHERE SDP.SESSION_ID = $1 AND SDP.DATA_STATUS IS NULL
		LIMIT $2
	`

	QStartSession = `
		INSERT INTO SESSION_DTL(SESSION_ID, KEY, NAME, VALUE)
		SELECT 
			S.ID, 'R1E', 'Ошибки', '0' 
		FROM SESSION S
		WHERE S.ID = $1 AND START_TIME IS NULL;
		
		UPDATE SESSION SET
			STATUS = "ST5",
			ERRORS = NULL,
			START_TIME = IIF(START_TIME IS NULL, CURRENT_TIMESTAMP, START_TIME) 
		WHERE ID = $2;	
	`

	QGetAllAliveSessions = `
	SELECT 
		ID ID, STATUS S, WORKER_COUNT W,
		timeout,
		COALESCE(DATABASE_ID, -1) DATABASE_ID, 
		COALESCE(MODULE_ID,   -1) MODULE_ID, 
		COALESCE(PROXY_ID,    -1) PROXY_ID,
		COALESCE(USERNAME_ID, -1) USERNAME_ID,
		COALESCE(PASSWORD_ID, -1) PASSWORD_ID
	FROM SESSION WHERE STATUS = "ST5" OR ID IN `

	QCehckAlreadyCreated = `
		SELECT 
			IIF(EXISTS(SELECT 1 FROM SESSION_DATA_PROTOCOL WHERE SESSION_ID = $1), 1, 0) 
		status;
	`

	QInsertProtocolLines = `
		INSERT INTO SESSION_DATA_PROTOCOL(SESSION_ID, ID, H_ID, H_DATA, U_ID, U_DATA, P_ID, P_DATA, DATA_CON_ID)
		WITH
			F AS (
				SELECT "TYPE", DATA
				FROM SESSION_COMBOLIST_TMP WHERE SESSION_ID = $1
			),
			H AS (
				SELECT ROW_NUMBER() OVER (ORDER BY 1) AS ID, DATA 
				FROM F WHERE "TYPE" = 'DATABASE_ID' 
			),
			U AS (
				SELECT ROW_NUMBER() OVER (ORDER BY 1) AS ID, DATA 
				FROM F WHERE "TYPE" = 'USERNAME_ID'
			),
			P AS (
				SELECT ROW_NUMBER() OVER (ORDER BY 1) AS ID, DATA 
				FROM F WHERE "TYPE" = 'PASSWORD_ID'
			)
		SELECT
			$2, ROW_NUMBER() OVER (ORDER BY 1), 
			H.ID, H.DATA, U.ID, U.DATA, P.ID, P.DATA,
			substr('0000000' || H.ID, -7, 7)||substr('000000' || U.ID, -6, 6)||substr('000000' || P.ID, -6, 6)
		FROM H
		JOIN U
		JOIN P;
		DELETE FROM SESSION_COMBOLIST_TMP WHERE SESSION_ID = $3;
	`

	QGetProxyInfo = `
		SELECT 
			P.INTERVAL, P.USE_UPDATE, 
			PL.ID, PL.LINK, PL.LINK_TYPE, PL.PROXY_TYPE 
		FROM PROXY P 
		INNER JOIN PROXY_LINK PL ON P.ID = PL.PROXY_ID
		WHERE P.ID = $1
	`

	QGetSessionProxys = "SELECT * FROM SESSION_PROXY WHERE SESSION_ID = $1"

	QSetSessionError = `
		UPDATE SESSION SET
			STATUS = 'ST3',
			ERRORS = $1
		WHERE ID = $2;
	`

	QGetProxys = `
		SELECT 
			PLD.HOST, PLD.PORT, PL.PROXY_TYPE type
		FROM PROXY_LINK_DATA PLD
		INNER JOIN PROXY_LINK PL ON PL.ID = PLD.PROXY_LINK
		WHERE PLD.PROXY_ID = $1 AND PLD.PROXY_LINK = $2
	`
)
