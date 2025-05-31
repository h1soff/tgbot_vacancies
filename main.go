package main

import (
    "bufio"
    "fmt"
    "log"
    "math/rand"
    "os"
    "runtime"
    "strconv"
    "strings"
    "sync"
    "time"

    tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Конфигурация
const (
    BotToken              = "тут апи тг бота"
    DataFolder            = "суда куда сохронятся логи"
    CalloutFolder         = DataFolder + "callout\\"
    UsersFile             = DataFolder + "users.txt"
    VacsFile              = DataFolder + "vacancies.txt"
    RespFile              = DataFolder + "responses.txt"
    CalloutsFile          = CalloutFolder + "callouts.txt"
    BotLogFile            = DataFolder + "bot.log"
    StatsLogFile          = DataFolder + "logsbot.txt"
    ForbiddenWordsFile    = DataFolder + "forbidden_words.txt"
    MaxUserID             = 5000
    MinUserID             = 1
    AdminUser1            = "тут админы"
    AdminUser2            = "тут 2 админ"
    VacancyExpirationDays = 7
    MaxCalloutLength      = 250
)

// Структуры данных
type User struct {
    Username      string
    ChatID        int64
    MinecraftNick string
    State         string
    UserID        int
    IsBanned      bool
    BanReason     string
    BanExpires    time.Time
    Bio           string
    Location      string
}

type Vacancy struct {
    ID           int
    Author       string
    Content      string
    Price        string
    PaymentInfo  string
    ChatID       int64
    Accepted     bool
    AcceptedBy   string
    AcceptedByID int64
    CreatedAt    time.Time
}

type Response struct {
    VacancyID int
    Responder string
    Message   string
}

type SupportMessage struct {
    UserID    int
    Username  string
    Nick      string
    Message   string
    Timestamp time.Time
}

type Callout struct {
    UserID    int
    Username  string
    Nick      string
    Message   string
    Timestamp time.Time
}

var (
    bot              *tgbotapi.BotAPI
    users            []User
    vacancies        []Vacancy
    responses        []Response
    supportMessages  []SupportMessage
    callouts         []Callout
    forbiddenWords   []string
    forbiddenWordsMu sync.RWMutex
    logFile          *os.File
    statsLogFile     *os.File
    tempVacancies    = make(map[int64]Vacancy)
    tempAlerts       = make(map[int64]string)
    nextVacancyID    = 1
    rng              = rand.New(rand.NewSource(time.Now().UnixNano()))
    banMutex         sync.Mutex
    userMutex        sync.Mutex
    startTime        = time.Now()
)

// Инициализация папки и лог-файлов
func initDataFolder() {
    if err := os.MkdirAll(DataFolder, os.ModePerm); err != nil {
        log.Fatal("Ошибка создания папки логов:", err)
    }
    if err := os.MkdirAll(CalloutFolder, os.ModePerm); err != nil {
        log.Fatal("Ошибка создания папки callout:", err)
    }

    var err error
    logFile, err = os.OpenFile(BotLogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
    if err != nil {
        log.Fatal("Ошибка открытия лог-файла:", err)
    }

    statsLogFile, err = os.OpenFile(StatsLogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
    if err != nil {
        log.Fatal("Ошибка открытия файла статистики:", err)
    }
}

// Загрузка пользователей
func loadUsers() {
    file, err := os.Open(UsersFile)
    if err != nil {
        return
    }
    defer file.Close()

    scanner := bufio.NewScanner(file)
    for scanner.Scan() {
        parts := strings.Split(scanner.Text(), "|")
        if len(parts) >= 8 {
            chatID, _ := strconv.ParseInt(parts[1], 10, 64)
            userID, _ := strconv.Atoi(parts[3])
            isBanned, _ := strconv.ParseBool(parts[4])
            banExpires, _ := time.Parse(time.RFC3339, parts[5])
            users = append(users, User{
                Username:      parts[0],
                ChatID:        chatID,
                MinecraftNick: parts[2],
                UserID:        userID,
                IsBanned:      isBanned,
                BanReason:     parts[6],
                BanExpires:    banExpires,
                Bio:           parts[7],
            })
        } else if len(parts) >= 7 {
            chatID, _ := strconv.ParseInt(parts[1], 10, 64)
            userID, _ := strconv.Atoi(parts[3])
            isBanned, _ := strconv.ParseBool(parts[4])
            banExpires, _ := time.Parse(time.RFC3339, parts[5])
            users = append(users, User{
                Username:      parts[0],
                ChatID:        chatID,
                MinecraftNick: parts[2],
                UserID:        userID,
                IsBanned:      isBanned,
                BanReason:     parts[6],
                BanExpires:    banExpires,
            })
        }
    }
}

// Сохранение пользователей
func saveUsers() {
    file, err := os.Create(UsersFile)
    if err != nil {
        logToFile("❌ Ошибка сохранения users.txt: " + err.Error())
        return
    }
    defer file.Close()

    for _, user := range users {
        banExpiresStr := ""
        if !user.BanExpires.IsZero() {
            banExpiresStr = user.BanExpires.Format(time.RFC3339)
        }
        _, err := file.WriteString(fmt.Sprintf("%s|%d|%s|%d|%t|%s|%s|%s\n", user.Username, user.ChatID, user.MinecraftNick, user.UserID, user.IsBanned, banExpiresStr, user.BanReason, user.Bio))
        if err != nil {
            logToFile("❌ Ошибка записи пользователя: " + err.Error())
        }
    }
}

// Загрузка вакансий
func loadVacancies() {
    file, err := os.Open(VacsFile)
    if err != nil {
        return
    }
    defer file.Close()

    scanner := bufio.NewScanner(file)
    for scanner.Scan() {
        parts := strings.Split(scanner.Text(), "|")
        if len(parts) >= 10 {
            id, _ := strconv.Atoi(parts[4])
            chatID, _ := strconv.ParseInt(parts[5], 10, 64)
            accepted, _ := strconv.ParseBool(parts[6])
            acceptedByID, _ := strconv.ParseInt(parts[8], 10, 64)
            createdAt, _ := time.Parse(time.RFC3339, parts[9])
            vacancies = append(vacancies, Vacancy{
                ID:           id,
                Author:       parts[0],
                Content:      parts[1],
                Price:        parts[2],
                PaymentInfo:  parts[3],
                ChatID:       chatID,
                Accepted:     accepted,
                AcceptedBy:   parts[7],
                AcceptedByID: acceptedByID,
                CreatedAt:    createdAt,
            })
            if id >= nextVacancyID {
                nextVacancyID = id + 1
            }
        }
    }
}

// Сохранение вакансий
func saveVacancies() {
    file, err := os.Create(VacsFile)
    if err != nil {
        logToFile("❌ Ошибка сохранения vacancies.txt: " + err.Error())
        return
    }
    defer file.Close()

    for _, vac := range vacancies {
        _, err := file.WriteString(fmt.Sprintf("%s|%s|%s|%s|%d|%d|%t|%s|%d|%s\n", vac.Author, vac.Content, vac.Price, vac.PaymentInfo, vac.ID, vac.ChatID, vac.Accepted, vac.AcceptedBy, vac.AcceptedByID, vac.CreatedAt.Format(time.RFC3339)))
        if err != nil {
            logToFile("❌ Ошибка записи вакансии: " + err.Error())
        }
    }
}

// Загрузка откликов
func loadResponses() {
    file, err := os.Open(RespFile)
    if err != nil {
        return
    }
    defer file.Close()

    scanner := bufio.NewScanner(file)
    for scanner.Scan() {
        parts := strings.Split(scanner.Text(), "|")
        if len(parts) >= 3 {
            vacID, _ := strconv.Atoi(parts[0])
            responses = append(responses, Response{
                VacancyID: vacID,
                Responder: parts[1],
                Message:   parts[2],
            })
        }
    }
}

// Сохранение откликов
func saveResponses() {
    file, err := os.Create(RespFile)
    if err != nil {
        logToFile("❌ Ошибка сохранения responses.txt: " + err.Error())
        return
    }
    defer file.Close()

    for _, resp := range responses {
        _, err := file.WriteString(fmt.Sprintf("%d|%s|%s\n", resp.VacancyID, resp.Responder, resp.Message))
        if err != nil {
            logToFile("❌ Ошибка записи отклика: " + err.Error())
        }
    }
}

// Загрузка отзывов
func loadCallouts() {
    file, err := os.Open(CalloutsFile)
    if err != nil {
        return
    }
    defer file.Close()

    scanner := bufio.NewScanner(file)
    for scanner.Scan() {
        parts := strings.Split(scanner.Text(), "|")
        if len(parts) >= 5 {
            userID, _ := strconv.Atoi(parts[0])
            timestamp, _ := time.Parse(time.RFC3339, parts[4])
            callouts = append(callouts, Callout{
                UserID:    userID,
                Username:  parts[1],
                Nick:      parts[2],
                Message:   parts[3],
                Timestamp: timestamp,
            })
        }
    }
}

// Сохранение отзывов
func saveCallouts() {
    file, err := os.Create(CalloutsFile)
    if err != nil {
        logToFile("❌ Ошибка сохранения callouts.txt: " + err.Error())
        return
    }
    defer file.Close()

    for _, callout := range callouts {
        _, err := file.WriteString(fmt.Sprintf("%d|%s|%s|%s|%s\n", callout.UserID, callout.Username, callout.Nick, callout.Message, callout.Timestamp.Format(time.RFC3339)))
        if err != nil {
            logToFile("❌ Ошибка записи отзыва: " + err.Error())
        }
    }
}

// Загрузка запрещённых слов
func loadForbiddenWords() {
    forbiddenWordsMu.Lock()
    defer forbiddenWordsMu.Unlock()

    if _, err := os.Stat(ForbiddenWordsFile); os.IsNotExist(err) {
        file, err := os.Create(ForbiddenWordsFile)
        if err != nil {
            logToFile("⚠️ Не удалось создать forbidden_words.txt: " + err.Error())
            return
        }
        defer file.Close()

        defaultWords := []string{"мат", "оскорбление", "дурак", "идиот"}
        for _, word := range defaultWords {
            if _, err := file.WriteString(word + "\n"); err != nil {
                logToFile("⚠️ Ошибка записи в forbidden_words.txt: " + err.Error())
                return
            }
        }
        forbiddenWords = defaultWords
        logToFile("✅ Создан forbidden_words.txt с начальными словами.")
        return
    }

    file, err := os.Open(ForbiddenWordsFile)
    if err != nil {
        logToFile("⚠️ Не удалось открыть forbidden_words.txt: " + err.Error())
        return
    }
    defer file.Close()

    forbiddenWords = nil
    scanner := bufio.NewScanner(file)
    for scanner.Scan() {
        word := strings.TrimSpace(scanner.Text())
        if word != "" {
            forbiddenWords = append(forbiddenWords, strings.ToLower(word))
        }
    }
    if err := scanner.Err(); err != nil {
        logToFile("⚠️ Ошибка чтения forbidden_words.txt: " + err.Error())
        return
    }
    logToFile(fmt.Sprintf("✅ Загружено %d запрещённых слов.", len(forbiddenWords)))
}

// Добавление запрещённого слова
func addForbiddenWord(word string) error {
    forbiddenWordsMu.Lock()
    defer forbiddenWordsMu.Unlock()

    word = strings.ToLower(strings.TrimSpace(word))
    for _, w := range forbiddenWords {
        if w == word {
            return fmt.Errorf("слово '%s' уже в списке", word)
        }
    }

    forbiddenWords = append(forbiddenWords, word)

    file, err := os.OpenFile(ForbiddenWordsFile, os.O_APPEND|os.O_WRONLY, 0644)
    if err != nil {
        return fmt.Errorf("не удалось открыть forbidden_words.txt: %v", err)
    }
    defer file.Close()

    if _, err := file.WriteString(word + "\n"); err != nil {
        return fmt.Errorf("ошибка записи в forbidden_words.txt: %v", err)
    }

    return nil
}

// Удаление запрещённого слова
func deleteForbiddenWord(word string) error {
    forbiddenWordsMu.Lock()
    defer forbiddenWordsMu.Unlock()

    word = strings.ToLower(strings.TrimSpace(word))
    foundIndex := -1
    for i, w := range forbiddenWords {
        if w == word {
            foundIndex = i
            break
        }
    }
    if foundIndex == -1 {
        return fmt.Errorf("слово '%s' не найдено в списке", word)
    }

    forbiddenWords = append(forbiddenWords[:foundIndex], forbiddenWords[foundIndex+1:]...)

    file, err := os.Create(ForbiddenWordsFile)
    if err != nil {
        return fmt.Errorf("не удалось открыть forbidden_words.txt: %v", err)
    }
    defer file.Close()

    for _, w := range forbiddenWords {
        if _, err := file.WriteString(w + "\n"); err != nil {
            return fmt.Errorf("ошибка записи в forbidden_words.txt: %v", err)
        }
    }

    return nil
}

// Проверка на запрещённые слова
func containsForbiddenWords(text string) (bool, string) {
    forbiddenWordsMu.RLock()
    defer forbiddenWordsMu.RUnlock()

    textLower := strings.ToLower(text)
    for _, word := range forbiddenWords {
        if strings.Contains(textLower, word) {
            return true, word
        }
    }
    return false, ""
}

// Логирование
func logToFile(message string) {
    timestamp := time.Now().Format("2006-01-02 15:04:05")
    if _, err := logFile.WriteString(fmt.Sprintf("[%s] %s\n", timestamp, message)); err != nil {
        log.Println("Ошибка записи в лог:", err)
    }
}

// Логирование статистики
func logStatsToFile(message string) {
    if _, err := statsLogFile.WriteString(message + "\n"); err != nil {
        log.Println("Ошибка записи в лог статистики:", err)
    }
}

// Очистка файла статистики
func clearStatsLogFile() {
    if err := os.Truncate(StatsLogFile, 0); err != nil {
        logToFile("❌ Ошибка очистки файла статистики: " + err.Error())
    } else {
        logToFile("🧹 Файл статистики logsbot.txt очищен.")
    }
}

// Мониторинг системы и удаление старых вакансий
func startSystemMonitoring() {
    statsTicker := time.NewTicker(1 * time.Minute)
    clearTicker := time.NewTicker(30 * time.Minute)
    cleanupTicker := time.NewTicker(24 * time.Hour)

    go func() {
        for {
            select {
            case <-statsTicker.C:
                logSystemStats()
            case <-clearTicker.C:
                clearStatsLogFile()
            case <-cleanupTicker.C:
                cleanupOldVacancies()
            }
        }
    }()
}

// Удаление старых вакансий
func cleanupOldVacancies() {
    expirationTime := time.Now().AddDate(0, 0, -VacancyExpirationDays)
    var newVacancies []Vacancy
    for _, vac := range vacancies {
        if !vac.Accepted && vac.CreatedAt.Before(expirationTime) {
            user := getUser(vac.ChatID)
            if user != nil {
                sendMsg(vac.ChatID, fmt.Sprintf("🗑 Вакансия #%d (%s) удалена, так как не была принята в течение %d дней.", vac.ID, vac.Content, VacancyExpirationDays))
            }
            logToFile(fmt.Sprintf("🗑 Удалена старая вакансия #%d от @%s (создана %s).", vac.ID, vac.Author, vac.CreatedAt.Format(time.DateTime)))
        } else {
            newVacancies = append(newVacancies, vac)
        }
    }
    vacancies = newVacancies
    saveVacancies()
}

// Системные метрики
func logSystemStats() {
    var memStats runtime.MemStats
    runtime.ReadMemStats(&memStats)

    stats := fmt.Sprintf(
        "[%s] Uptime: %s | HeapAlloc: %d MB | TotalAlloc: %d MB | SysMemory: %d MB | Goroutines: %d | Users: %d | Vacancies: %d | Responses: %d | Callouts: %d",
        time.Now().Format("2006-01-02 15:04:05"), time.Since(startTime).String(),
        memStats.HeapAlloc/1024/1024, memStats.TotalAlloc/1024/1024, memStats.Sys/1024/1024,
        runtime.NumGoroutine(), len(users), len(vacancies), len(responses), len(callouts),
    )
    logStatsToFile(stats)
}

// Основная функция
func main() {
    initDataFolder()
    loadUsers()
    loadVacancies()
    loadResponses()
    loadCallouts()
    loadForbiddenWords()
    startSystemMonitoring()

    var err error
    bot, err = tgbotapi.NewBotAPI(BotToken)
    if err != nil {
        log.Fatal("Ошибка подключения к боту:", err)
    }
    defer logFile.Close()
    defer statsLogFile.Close()

    bot.Debug = true
    logToFile(fmt.Sprintf("🤖 Бот запущен: @%s", bot.Self.UserName))

    updates := bot.GetUpdatesChan(tgbotapi.NewUpdate(0))
    for update := range updates {
        if update.Message == nil {
            continue
        }

        chatID := update.Message.Chat.ID
        text := update.Message.Text
        username := update.Message.From.UserName

        logToFile(fmt.Sprintf("%s: %s", username, text))

        user := getUser(chatID)
        if user != nil && user.IsBanned && time.Now().Before(user.BanExpires) {
            sendMsg(chatID, fmt.Sprintf("🚫 Вы заблокированы. Причина: %s. Блокировка истекает: %s", user.BanReason, user.BanExpires.Format(time.DateTime)))
            continue
        } else if user != nil && user.IsBanned && time.Now().After(user.BanExpires) {
            unbanUser(user)
            sendMsg(chatID, "🔓 Срок вашей блокировки истек.")
        }

        if user != nil && user.State != "" {
            handleUserState(chatID, update.Message, user)
            continue
        }

        switch {
        case text == "/start":
            sendMsg(chatID, "👋 Добро пожаловать! Введите /help для списка команд.")
        case text == "/help":
            sendHelp(chatID, username)
        case text == "/register":
            startRegistration(chatID, username)
        case text == "/create":
            startVacancyCreation(chatID, username)
        case text == "/list_users":
            listUsers(chatID, username)
        case strings.HasPrefix(text, "/list"):
            parts := strings.SplitN(text, " ", 2)
            page := 1
            if len(parts) == 2 {
                page, _ = strconv.Atoi(parts[1])
                if page < 1 {
                    page = 1
                }
            }
            sendVacanciesList(chatID, page)
        case strings.HasPrefix(text, "/Оповищения"):
            sendAnnouncement(chatID, text, username)
        case strings.HasPrefix(text, "/Alerts"):
            processAlertsCommand(chatID, text, username)
        case strings.HasPrefix(text, "/support"):
            processSupportCommand(chatID, text, username)
        case strings.HasPrefix(text, "/reply"):
            processReplyCommand(chatID, text, username)
        case strings.HasPrefix(text, "Отклик:"):
            processResponse(chatID, text, username)
        case strings.HasPrefix(text, "!"):
            processAcceptOrder(chatID, text)
        case strings.HasPrefix(text, "/chat"):
            processChatCommand(chatID, text)
        case strings.HasPrefix(text, "/ban_user"):
            processBanUserCommand(chatID, text, username)
        case text == "lovs":
            clearLogFile()
            sendMsg(chatID, "Лог-файл очищен.")
        case text == "/sell_lot_poi_good22366552998":
            removeAllVacancies(chatID, username)
        case text == "/sell_lot_poi_good2236655299865541111976hhffrtt":
            removeAllUsers(chatID, username)
        case strings.HasPrefix(text, "/change_id"):
            processChangeIDCommand(chatID, text, username)
        case strings.HasPrefix(text, "/change_nick"):
            processChangeNickCommand(chatID, text, username)
        case strings.HasPrefix(text, "/dell_sell333"):
            processDeleteVacancyCommand(chatID, text, username)
        case text == "/profile":
            showUserProfile(chatID)
        case text == "/my_vacancies":
            showMyVacancies(chatID)
        case strings.HasPrefix(text, "/delete_vacancy"):
            deleteMyVacancy(chatID, text)
        case strings.HasPrefix(text, "/del_user"):
            deleteUser(chatID, text, username)
        case strings.HasPrefix(text, "/unban_user"):
            unbanUserByAdmin(chatID, text, username)
        case text == "/restart_bot":
            restartBot(chatID, username)
        case text == "/version":
            showVersion(chatID)
        case strings.HasPrefix(text, "/set_bio"):
            processSetBioCommand(chatID, text, username)
        case strings.HasPrefix(text, "/banwords"):
            processBanWordsCommand(chatID, text, username)
        case strings.HasPrefix(text, "/delbanword"):
            processDelBanWordCommand(chatID, text, username)
        case strings.HasPrefix(text, "/callout"):
            processCalloutCommand(chatID, text, username)
        default:
            if tryProcessVacancyInfo(chatID, text) {
                continue
            }
            sendMsg(chatID, "❌ Неизвестная команда. Введите /help.")
        }
    }
}

// Получение пользователя
func getUser(chatID int64) *User {
    userMutex.Lock()
    defer userMutex.Unlock()
    for i, user := range users {
        if user.ChatID == chatID {
            return &users[i]
        }
    }
    return nil
}

func getUserByUserID(userID int) *User {
    userMutex.Lock()
    defer userMutex.Unlock()
    for i, user := range users {
        if user.UserID == userID {
            return &users[i]
        }
    }
    return nil
}

func getUserByUsername(username string) *User {
    userMutex.Lock()
    defer userMutex.Unlock()
    for i, user := range users {
        if user.Username == username {
            return &users[i]
        }
    }
    return nil
}

// Регистрация
func startRegistration(chatID int64, username string) {
    if getUser(chatID) != nil {
        sendMsg(chatID, "❌ Вы уже зарегистрированы!")
        return
    }

    newUser := User{
        Username:      username,
        ChatID:        chatID,
        State:         "awaiting_nick",
        MinecraftNick: "",
        UserID:        generateUserID(),
        Bio:           "",
    }
    userMutex.Lock()
    users = append(users, newUser)
    userMutex.Unlock()
    saveUsers()
    sendMsg(chatID, "Введите свой ник Minecraft:")
}

// Обработка состояний
func handleUserState(chatID int64, message *tgbotapi.Message, user *User) {
    switch user.State {
    case "awaiting_nick":
        if isNickTaken(message.Text) {
            sendMsg(chatID, "❌ Этот ник уже занят.")
            return
        }
        user.MinecraftNick = message.Text
        user.State = ""
        saveUsers()
        sendMsg(chatID, fmt.Sprintf("✅ Регистрация завершена! Ник: %s, ID: %d", message.Text, user.UserID))
    case "awaiting_vacancy_content":
        if hasForbidden, word := containsForbiddenWords(message.Text); hasForbidden {
            sendMsg(chatID, fmt.Sprintf("❌ Сообщение содержит запрещённое слово: %s.", word))
            logToFile(fmt.Sprintf("🚫 @%s пытался использовать '%s' в вакансии.", user.Username, word))
            return
        }
        tempVacancies[chatID] = Vacancy{
            Author:      user.MinecraftNick,
            Content:     message.Text,
            ChatID:      chatID,
            PaymentInfo: "",
            CreatedAt:   time.Now(),
        }
        user.State = "awaiting_vacancy_price"
        sendMsg(chatID, "2. Сколько вы предлагаете? (например, 2 алмаза)")
    case "awaiting_vacancy_price":
        if hasForbidden, word := containsForbiddenWords(message.Text); hasForbidden {
            sendMsg(chatID, fmt.Sprintf("❌ Сообщение содержит запрещённое слово: %s.", word))
            logToFile(fmt.Sprintf("🚫 @%s пытался использовать '%s' в цене.", user.Username, word))
            return
        }
        if vac, ok := tempVacancies[chatID]; ok {
            vac.Price = message.Text
            tempVacancies[chatID] = vac
            user.State = "awaiting_vacancy_payment"
            sendMsg(chatID, "3. Куда и как производить оплату? (например, сундук на x:100, y:64, z:200)")
        }
    case "awaiting_vacancy_payment":
        if hasForbidden, word := containsForbiddenWords(message.Text); hasForbidden {
            sendMsg(chatID, fmt.Sprintf("❌ Сообщение содержит запрещённое слово: %s.", word))
            logToFile(fmt.Sprintf("🚫 @%s пытался использовать '%s' в оплате.", user.Username, word))
            return
        }
        if vac, ok := tempVacancies[chatID]; ok {
            vac.PaymentInfo = message.Text
            vac.ID = nextVacancyID
            vacancies = append(vacancies, vac)
            saveVacancies()
            nextVacancyID++

            notifyAllUsers(fmt.Sprintf(
                "📢 Новая вакансия!\nОт: %s\nНужно: %s\nЦена: %s\nОплата: %s\nID: #%d",
                vac.Author, vac.Content, vac.Price, vac.PaymentInfo, vac.ID,
            ))

            delete(tempVacancies, chatID)
            user.State = ""
            sendMsg(chatID, "✅ Вакансия создана!")
        }
    case "awaiting_alert_photo":
        if message.Photo == nil || len(message.Photo) == 0 {
            sendMsg(chatID, "❌ Отправьте фото.")
            return
        }
        alertText, ok := tempAlerts[chatID]
        if !ok {
            sendMsg(chatID, "❌ Ошибка: текст объявления не найден.")
            user.State = ""
            return
        }
        photo := message.Photo[len(message.Photo)-1]
        notifyAllUsersWithPhoto(fmt.Sprintf("📢 Объявление:\n%s", alertText), photo.FileID)
        sendMsg(chatID, "✅ Объявление с фото отправлено.")
        logToFile(fmt.Sprintf("Админ @%s отправил объявление с фото: %s", user.Username, alertText))
        delete(tempAlerts, chatID)
        user.State = ""
    }
}

// Создание вакансии
func startVacancyCreation(chatID int64, username string) {
    user := getUser(chatID)
    if user == nil {
        sendMsg(chatID, "❌ Сначала зарегистрируйтесь (/register).")
        return
    }
    if user.IsBanned {
        sendMsg(chatID, "❌ Вы заблокированы: " + user.BanReason)
        return
    }
    if user.MinecraftNick == "" {
        sendMsg(chatID, "❌ У вас не установлен ник Minecraft.")
        return
    }
    user.State = "awaiting_vacancy_content"
    sendMsg(chatID, "1. Что вам нужно? (например: 32 стопки мха)")
}

// Уведомления
func notifyAllUsers(message string) {
    for _, user := range users {
        sendMsg(user.ChatID, message)
    }
}

func notifyAllUsersWithPhoto(message string, photoFileID string) {
    for _, user := range users {
        msg := tgbotapi.NewPhoto(user.ChatID, tgbotapi.FileID(photoFileID))
        msg.Caption = message
        if _, err := bot.Send(msg); err != nil {
            logToFile(fmt.Sprintf("❌ Ошибка отправки фото @%s: %s", user.Username, err.Error()))
        }
    }
}

// Объявления
func sendAnnouncement(chatID int64, text string, username string) {
    if username != AdminUser1 && username != AdminUser2 {
        sendMsg(chatID, "❌ У вас нет прав.")
        return
    }
    parts := strings.SplitN(text, " ", 2)
    if len(parts) != 2 {
        sendMsg(chatID, "❌ Формат: /Оповищения [сообщение]")
        return
    }
    announcement := strings.TrimSpace(parts[1])
    if announcement == "" {
        sendMsg(chatID, "❌ Сообщение не может быть пустым.")
        return
    }
    if hasForbidden, word := containsForbiddenWords(announcement); hasForbidden {
        sendMsg(chatID, fmt.Sprintf("❌ Сообщение содержит запрещённое слово: %s.", word))
        logToFile(fmt.Sprintf("🚫 Админ @%s пытался использовать '%s' в объявлении.", username, word))
        return
    }
    logToFile(fmt.Sprintf("Админ @%s отправил объявление: %s", username, announcement))
    notifyAllUsers(fmt.Sprintf("📢 Объявление:\n%s", announcement))
    sendMsg(chatID, "✅ Объявление отправлено.")
}

func processAlertsCommand(chatID int64, text string, username string) {
    if username != AdminUser1 && username != AdminUser2 {
        sendMsg(chatID, "❌ У вас нет прав.")
        return
    }
    parts := strings.SplitN(text, " ", 2)
    if len(parts) != 2 {
        sendMsg(chatID, "❌ Формат: /Alerts [сообщение]")
        return
    }
    alertText := strings.TrimSpace(parts[1])
    if alertText == "" {
        sendMsg(chatID, "❌ Сообщение не может быть пустым.")
        return
    }
    if hasForbidden, word := containsForbiddenWords(alertText); hasForbidden {
        sendMsg(chatID, fmt.Sprintf("❌ Сообщение содержит запрещённое слово: %s.", word))
        logToFile(fmt.Sprintf("🚫 Админ @%s пытался использовать '%s' в объявлении с фото.", username, word))
        return
    }
    user := getUser(chatID)
    if user == nil {
        sendMsg(chatID, "❌ Вы не зарегистрированы.")
        return
    }
    tempAlerts[chatID] = alertText
    user.State = "awaiting_alert_photo"
    sendMsg(chatID, "📸 Отправьте фото.")
}

// Техподдержка
func processSupportCommand(chatID int64, text string, username string) {
    user := getUser(chatID)
    if user == nil {
        sendMsg(chatID, "❌ Зарегистрируйтесь (/register).")
        return
    }
    parts := strings.SplitN(text, " ", 2)
    if len(parts) != 2 {
        sendMsg(chatID, "❌ Формат: /support [сообщение]")
        return
    }
    supportText := strings.TrimSpace(parts[1])
    if supportText == "" {
        sendMsg(chatID, "❌ Сообщение не может быть пустым.")
        return
    }
    if hasForbidden, word := containsForbiddenWords(supportText); hasForbidden {
        sendMsg(chatID, fmt.Sprintf("❌ Сообщение содержит запрещённое слово: %s.", word))
        logToFile(fmt.Sprintf("🚫 @%s пытался использовать '%s' в техподдержке.", username, word))
        return
    }
    supportMsg := SupportMessage{
        UserID:    user.UserID,
        Username:  user.Username,
        Nick:      user.MinecraftNick,
        Message:   supportText,
        Timestamp: time.Now(),
    }
    supportMessages = append(supportMessages, supportMsg)

    for _, admin := range []string{AdminUser1, AdminUser2} {
        adminUser := getUserByUsername(admin)
        if adminUser != nil {
            sendMsg(adminUser.ChatID, fmt.Sprintf("🆘 Обращение от @%s (ID: %d, Ник: %s):\n%s", user.Username, user.UserID, user.MinecraftNick, supportText))
        }
    }
    sendMsg(chatID, "✅ Обращение отправлено.")
    logToFile(fmt.Sprintf("Обращение от @%s (ID: %d): %s", user.Username, user.UserID, supportText))
}

func processReplyCommand(chatID int64, text string, username string) {
    if username != AdminUser1 && username != AdminUser2 {
        sendMsg(chatID, "❌ У вас нет прав.")
        return
    }
    parts := strings.SplitN(text, " ", 3)
    if len(parts) != 3 {
        sendMsg(chatID, "❌ Формат: /reply [ID_пользователя] [сообщение]")
        return
    }
    targetUserID, err := strconv.Atoi(parts[1])
    if err != nil {
        sendMsg(chatID, "❌ Некорректный ID.")
        return
    }
    replyText := strings.TrimSpace(parts[2])
    if replyText == "" {
        sendMsg(chatID, "❌ Сообщение не может быть пустым.")
        return
    }
    targetUser := getUserByUserID(targetUserID)
    if targetUser == nil {
        sendMsg(chatID, fmt.Sprintf("❌ Пользователь с ID %d не найден.", targetUserID))
        return
    }
    sendMsg(targetUser.ChatID, fmt.Sprintf("📩 Ответ техподдержки:\n%s", replyText))
    sendMsg(chatID, fmt.Sprintf("✅ Ответ отправлен @%s (ID: %d).", targetUser.Username, targetUser.UserID))
    logToFile(fmt.Sprintf("Ответ от @%s пользователю @%s (ID: %d): %s", username, targetUser.Username, targetUser.UserID, replyText))
}

// Обработка отзыва
func processCalloutCommand(chatID int64, text string, username string) {
    user := getUser(chatID)
    if user == nil {
        sendMsg(chatID, "❌ Зарегистрируйтесь (/register).")
        return
    }
    parts := strings.SplitN(text, " ", 2)
    if len(parts) != 2 {
        sendMsg(chatID, "❌ Формат: /callout [отзыв]")
        return
    }
    calloutText := strings.TrimSpace(parts[1])
    if calloutText == "" {
        sendMsg(chatID, "❌ Отзыв не может быть пустым.")
        return
    }
    if len(calloutText) > MaxCalloutLength {
        sendMsg(chatID, fmt.Sprintf("❌ Отзыв слишком длинный (макс. %d символов).", MaxCalloutLength))
        return
    }
    callout := Callout{
        UserID:    user.UserID,
        Username:  user.Username,
        Nick:      user.MinecraftNick,
        Message:   calloutText,
        Timestamp: time.Now(),
    }
    callouts = append(callouts, callout)
    saveCallouts()

    for _, admin := range []string{AdminUser1, AdminUser2} {
        adminUser := getUserByUsername(admin)
        if adminUser != nil {
            sendMsg(adminUser.ChatID, fmt.Sprintf("📢 Новый отзыв от @%s (ID: %d, Ник: %s):\n%s", user.Username, user.UserID, user.MinecraftNick, calloutText))
        }
    }
    sendMsg(chatID, "✅ Спасибо за отзыв!")
    logToFile(fmt.Sprintf("📢 Отзыв от @%s (ID: %d): %s", user.Username, user.UserID, calloutText))
}

// Установка описания профиля
func processSetBioCommand(chatID int64, text string, username string) {
    user := getUser(chatID)
    if user == nil {
        sendMsg(chatID, "❌ Зарегистрируйтесь (/register).")
        return
    }
    parts := strings.SplitN(text, " ", 2)
    if len(parts) != 2 {
        sendMsg(chatID, "❌ Формат: /set_bio [описание]")
        return
    }
    bio := strings.TrimSpace(parts[1])
    if bio == "" {
        sendMsg(chatID, "❌ Описание не может быть пустым.")
        return
    }
    if len(bio) > 100 {
        sendMsg(chatID, "❌ Описание слишком длинное (макс. 100 символов).")
        return
    }
    if hasForbidden, word := containsForbiddenWords(bio); hasForbidden {
        sendMsg(chatID, fmt.Sprintf("❌ Описание содержит запрещённое слово: %s.", word))
        logToFile(fmt.Sprintf("🚫 @%s пытался использовать '%s' в описании профиля.", username, word))
        return
    }
    userMutex.Lock()
    user.Bio = bio
    userMutex.Unlock()
    saveUsers()
    sendMsg(chatID, fmt.Sprintf("✅ Описание профиля: %s", bio))
    logToFile(fmt.Sprintf("@%s (ID: %d) обновил описание: %s", username, user.UserID, bio))
}

// Добавление запрещённых слов
func processBanWordsCommand(chatID int64, text string, username string) {
    if username != AdminUser1 && username != AdminUser2 {
        sendMsg(chatID, "❌ У вас нет прав.")
        return
    }
    parts := strings.SplitN(text, " ", 2)
    if len(parts) != 2 {
        sendMsg(chatID, "❌ Формат: /banwords [слово]")
        return
    }
    word := strings.TrimSpace(parts[1])
    if word == "" {
        sendMsg(chatID, "❌ Слово не может быть пустым.")
        return
    }
    if err := addForbiddenWord(word); err != nil {
        sendMsg(chatID, fmt.Sprintf("❌ Ошибка: %s.", err))
        return
    }
    sendMsg(chatID, fmt.Sprintf("✅ Слово '%s' добавлено в запрещённые.", word))
    logToFile(fmt.Sprintf("Админ @%s добавил запрещённое слово: %s", username, word))
}

// Удаление запрещённых слов
func processDelBanWordCommand(chatID int64, text string, username string) {
    if username != AdminUser1 && username != AdminUser2 {
        sendMsg(chatID, "❌ У вас нет прав.")
        return
    }
    parts := strings.SplitN(text, " ", 2)
    if len(parts) != 2 {
        sendMsg(chatID, "❌ Формат: /delbanword [слово]")
        return
    }
    word := strings.TrimSpace(parts[1])
    if word == "" {
        sendMsg(chatID, "❌ Слово не может быть пустым.")
        return
    }
    if err := deleteForbiddenWord(word); err != nil {
        sendMsg(chatID, fmt.Sprintf("❌ Ошибка: %s.", err))
        return
    }
    sendMsg(chatID, fmt.Sprintf("✅ Слово '%s' удалено из запрещённых.", word))
    logToFile(fmt.Sprintf("Админ @%s удалил запрещённое слово: %s", username, word))
}

// Список вакансий
func sendVacanciesList(chatID int64, page int) {
    const itemsPerPage = 10
    if len(vacancies) == 0 {
        sendMsg(chatID, "ℹ️ Нет вакансий.")
        return
    }
    startIndex := (page - 1) * itemsPerPage
    if startIndex >= len(vacancies) {
        sendMsg(chatID, fmt.Sprintf("❌ Страница %d не существует.", page))
        return
    }
    var result strings.Builder
    result.WriteString(fmt.Sprintf("📋 Вакансии (Страница %d):\n", page))
    endIndex := startIndex + itemsPerPage
    if endIndex > len(vacancies) {
        endIndex = len(vacancies)
    }
    for i := startIndex; i < endIndex; i++ {
        vac := vacancies[i]
        acceptedStr := "❌ Не принята"
        if vac.Accepted {
            acceptedStr = fmt.Sprintf("✅ Принята: %s", vac.AcceptedBy)
        }
        paymentInfo := vac.PaymentInfo
        if paymentInfo == "" {
            paymentInfo = "Не указано"
        }
        result.WriteString(fmt.Sprintf("#%d | От: %s | Нужно: %s | Цена: %s | Оплата: %s | Статус: %s\n", vac.ID, vac.Author, vac.Content, vac.Price, paymentInfo, acceptedStr))
    }
    if len(vacancies) > itemsPerPage {
        result.WriteString(fmt.Sprintf("\n📄 Показано %d-%d из %d. Используйте /list [страница].", startIndex+1, endIndex, len(vacancies)))
    }
    sendMsg(chatID, result.String())
}

// Отклики
func processResponse(chatID int64, text string, responder string) {
    parts := strings.SplitN(text, ":", 2)
    if len(parts) != 2 {
        sendMsg(chatID, "❌ Формат: 'Отклик: [ID_вакансии] [предложение]'")
        return
    }
    responseParts := strings.SplitN(strings.TrimSpace(parts[1]), " ", 2)
    if len(responseParts) < 2 {
        sendMsg(chatID, "❌ Укажите ID и текст отклика.")
        return
    }
    vacIDStr := responseParts[0]
    responseMsg := responseParts[1]
    if hasForbidden, word := containsForbiddenWords(responseMsg); hasForbidden {
        sendMsg(chatID, fmt.Sprintf("❌ Отклик содержит запрещённое слово: %s.", word))
        logToFile(fmt.Sprintf("🚫 @%s пытался использовать '%s' в отклике.", responder, word))
        return
    }
    vacID, err := strconv.Atoi(vacIDStr)
    if err != nil {
        sendMsg(chatID, "❌ Некорректный ID вакансии.")
        return
    }
    found := false
    var vacancyAuthorChatID int64
    var vacancyIndex int
    for i, vac := range vacancies {
        if vac.ID == vacID {
            found = true
            vacancyAuthorChatID = vac.ChatID
            vacancyIndex = i
            break
        }
    }
    if !found {
        sendMsg(chatID, fmt.Sprintf("❌ Вакансия #%d не найдена.", vacID))
        return
    }
    if vacancies[vacancyIndex].Accepted {
        sendMsg(chatID, "❌ Вакансия уже принята.")
        return
    }
    response := Response{
        VacancyID: vacID,
        Responder: responder,
        Message:   responseMsg,
    }
    responses = append(responses, response)
    saveResponses()
    sendMsg(chatID, fmt.Sprintf("✅ Отклик на вакансию #%d принят!", vacID))
    vacancies[vacancyIndex].Accepted = true
    vacancies[vacancyIndex].AcceptedBy = getUser(chatID).MinecraftNick
    vacancies[vacancyIndex].AcceptedByID = chatID
    saveVacancies()
    if vacancyAuthorChatID != 0 {
        responderUser := getUser(chatID)
        if responderUser != nil {
            sendMsg(vacancyAuthorChatID, fmt.Sprintf("✉️ Вакансия #%d принята @%s (%s)! Связаться: /chat %d (ID: %d)", vacID, responder, responderUser.MinecraftNick, chatID, responderUser.UserID))
        }
    }
}

// Удаление данных
func removeAllUsers(chatID int64, username string) {
    if username != AdminUser1 && username != AdminUser2 {
        sendMsg(chatID, "❌ У вас нет прав.")
        return
    }
    userMutex.Lock()
    users = []User{}
    userMutex.Unlock()
    saveUsers()
    sendMsg(chatID, "✅ Все пользователи удалены.")
}

func removeAllVacancies(chatID int64, username string) {
    if username != AdminUser1 && username != AdminUser2 {
        sendMsg(chatID, "❌ У вас нет прав.")
        return
    }
    vacancies = []Vacancy{}
    saveVacancies()
    sendMsg(chatID, "✅ Все вакансии удалены.")
}

func clearLogFile() {
    if err := os.Truncate(BotLogFile, 0); err != nil {
        logToFile("❌ Ошибка очистки лога: " + err.Error())
    }
}

// Отправка сообщения
func sendMsg(chatID int64, text string) {
    msg := tgbotapi.NewMessage(chatID, text)
    if _, err := bot.Send(msg); err != nil {
        logToFile("❌ Ошибка отправки: " + err.Error())
        if strings.Contains(err.Error(), "blocked by user") {
            userMutex.Lock()
            for i, user := range users {
                if user.ChatID == chatID {
                    users = append(users[:i], users[i+1:]...)
                    saveUsers()
                    logToFile(fmt.Sprintf("❌ @%s (ID: %d) удалён (заблокировал бота).", user.Username, user.UserID))
                    break
                }
            }
            userMutex.Unlock()
        }
    }
}

// Справка
func sendHelp(chatID int64, username string) {
    helpText := `
🎮 Команды бота:
📝 /register — Зарегистрироваться
🛠 /create — Создать вакансию
📋 /list [страница] — Список вакансий
📂 /my_vacancies — Ваши вакансии
🗑 /delete_vacancy [ID] — Удалить свою вакансию
👤 /profile — Ваш профиль
✏️ /set_bio [описание] — Установить описание (до 100 символов)
💬 /chat [ID_пользователя] — Начать диалог
🆘 /support [сообщение] — Техподдержка
📢 /callout [отзыв] — Оставить отзыв о сервере (до 250 символов)
ℹ️ /version — Версия бота
❓ /help — Справка
Для принятия: ![ID_заказа]
`
    adminHelpText := `
👑 Админ-команды:
📄 /list_users — Список пользователей
🚫 /ban_user [ID] [время_мин]мин [причина] — Забанить
✅ /unban_user [ID] — Разбанить
❌ /del_user [ID] [причина] — Удалить пользователя
🔄 /change_id [ID] [новый_ID] — Изменить ID
✏️ /change_nick [ID] [новый_ник] — Изменить ник
🗑 /dell_sell333 [ID_вакансии] — Удалить вакансию
📢 /Оповищения [сообщение] — Текстовое объявление
🖼 /Alerts [сообщение] — Объявление с фото
📩 /reply [ID_пользователя] [сообщение] — Ответ техподдержки
🚫 /banwords [слово] — Добавить запрещённое слово
✅ /delbanword [слово] — Удалить запрещённое слово
🔁 /restart_bot — Перезапустить бота
🧹 lovs — Очистить лог
💣 sell_lot_poi_good22366552998 — Удалить все вакансии
☠️ sell_lot_poi_good2236655299865541111976hhffrtt — Удалить всех пользователей
`
    if username == AdminUser1 || username == AdminUser2 {
        sendMsg(chatID, helpText+adminHelpText)
    } else {
        sendMsg(chatID, helpText)
    }
}

// Обработка вакансий
func tryProcessVacancyInfo(chatID int64, text string) bool {
    parts := strings.Split(text, "|")
    if len(parts) != 4 {
        return false
    }
    idStr := strings.TrimSpace(strings.TrimPrefix(parts[0], "#"))
    content := strings.TrimSpace(strings.TrimPrefix(parts[2], "Нужно: "))
    price := strings.TrimSpace(strings.TrimPrefix(parts[3], "Цена: "))
    if hasForbidden, word := containsForbiddenWords(content+" "+price); hasForbidden {
        sendMsg(chatID, fmt.Sprintf("❌ Предложение содержит запрещённое слово: %s.", word))
        logToFile(fmt.Sprintf("🚫 @%s пытался использовать '%s' в предложении.", getUser(chatID).Username, word))
        return true
    }
    id, err := strconv.Atoi(idStr)
    if err != nil {
        sendMsg(chatID, "❌ Некорректный ID.")
        return true
    }
    found := false
    var vacancyAuthorChatID int64
    for _, vac := range vacancies {
        if vac.ID == id {
            found = true
            vacancyAuthorChatID = vac.ChatID
            break
        }
    }
    if !found {
        sendMsg(chatID, fmt.Sprintf("❌ Вакансия #%d не найдена.", id))
        return true
    }
    response := Response{
        VacancyID: id,
        Responder: getUser(chatID).MinecraftNick,
        Message:   fmt.Sprintf("Предлагаю: %s, Цена: %s", content, price),
    }
    responses = append(responses, response)
    saveResponses()
    sendMsg(chatID, fmt.Sprintf("✅ Предложение по вакансии #%d принято.", id))
    if vacancyAuthorChatID != 0 {
        responderUser := getUser(chatID)
        if responderUser != nil {
            sendMsg(vacancyAuthorChatID, fmt.Sprintf("✉️ Предложение на вакансию #%d от %s (ID: %d): %s", id, responderUser.MinecraftNick, responderUser.UserID, response.Message))
        }
    }
    return true
}

func processAcceptOrder(chatID int64, text string) {
    vacIDStr := strings.TrimSpace(strings.TrimPrefix(text, "!"))
    vacID, err := strconv.Atoi(vacIDStr)
    if err != nil {
        sendMsg(chatID, "❌ Некорректный ID.")
        return
    }
    found := false
    var vacancyIndex int
    for i, vac := range vacancies {
        if vac.ID == vacID {
            found = true
            vacancyIndex = i
            break
        }
    }
    if !found {
        sendMsg(chatID, fmt.Sprintf("❌ Вакансия #%d не найдена.", vacID))
        return
    }
    if vacancies[vacancyIndex].Accepted {
        sendMsg(chatID, "❌ Вакансия уже принята.")
        return
    }
    vacancies[vacancyIndex].Accepted = true
    vacancies[vacancyIndex].AcceptedBy = getUser(chatID).MinecraftNick
    vacancies[vacancyIndex].AcceptedByID = chatID
    saveVacancies()
    sendMsg(chatID, fmt.Sprintf("✅ Заказ #%d принят!", vacID))
    if vacancies[vacancyIndex].ChatID != 0 {
        acceptorUser := getUser(chatID)
        if acceptorUser != nil {
            sendMsg(vacancies[vacancyIndex].ChatID, fmt.Sprintf("✉️ Заказ #%d принят @%s (%s)! Связаться: /chat %d (ID: %d)", vacID, acceptorUser.Username, acceptorUser.MinecraftNick, chatID, acceptorUser.UserID))
        }
    }
}

// Проверка ника
func isNickTaken(nick string) bool {
    userMutex.Lock()
    defer userMutex.Unlock()
    for _, user := range users {
        if strings.EqualFold(user.MinecraftNick, nick) {
            return true
        }
    }
    return false
}

// Генерация ID
func generateUserID() int {
    return rng.Intn(MaxUserID-MinUserID+1) + MinUserID
}

// Чат
func processChatCommand(chatID int64, text string) {
    parts := strings.SplitN(text, " ", 2)
    if len(parts) != 2 {
        sendMsg(chatID, "❌ Формат: /chat [ID_пользователя]")
        return
    }
    targetUserID, err := strconv.Atoi(parts[1])
    if err != nil {
        sendMsg(chatID, "❌ Некорректный ID.")
        return
    }
    targetUser := getUserByUserID(targetUserID)
    if targetUser == nil {
        sendMsg(chatID, fmt.Sprintf("❌ Пользователь с ID %d не найден.", targetUserID))
        return
    }
    if targetUser.ChatID == chatID {
        sendMsg(chatID, "❌ Нельзя начать чат с собой.")
        return
    }
    currentUser := getUser(chatID)
    if currentUser != nil {
        sendMsg(chatID, fmt.Sprintf("✅ Чат с @%s (ID: %d) начат! Ваш ID: %d", targetUser.Username, targetUser.UserID, currentUser.UserID))
        sendMsg(targetUser.ChatID, fmt.Sprintf("✅ Чат начат с @%s (ID: %d)! Ваш ID: %d", currentUser.Username, currentUser.UserID, targetUser.UserID))
    }
}

// Бан
func processBanUserCommand(chatID int64, text string, username string) {
    if username != AdminUser1 && username != AdminUser2 {
        sendMsg(chatID, "❌ У вас нет прав.")
        return
    }
    parts := strings.SplitN(text, " ", 3)
    if len(parts) != 3 {
        sendMsg(chatID, "❌ Формат: /ban_user [ID] [время_мин]мин [причина]")
        return
    }
    targetUserID, err := strconv.Atoi(parts[1])
    if err != nil {
        sendMsg(chatID, "❌ Некорректный ID.")
        return
    }
    banDurationStr := strings.TrimSuffix(parts[2], "мин")
    banDurationParts := strings.SplitN(banDurationStr, " ", 2)
    if len(banDurationParts) != 2 {
        sendMsg(chatID, "❌ Формат времени: [время_мин]мин [причина]")
        return
    }
    banDurationMinutes, err := strconv.Atoi(banDurationParts[0])
    if err != nil {
        sendMsg(chatID, "❌ Некорректное время.")
        return
    }
    banReason := strings.TrimSpace(banDurationParts[1])
    if banReason == "" {
        sendMsg(chatID, "❌ Укажите причину.")
        return
    }
    targetUser := getUserByUserID(targetUserID)
    if targetUser == nil {
        sendMsg(chatID, fmt.Sprintf("❌ Пользователь с ID %d не найден.", targetUserID))
        return
    }
    if targetUser.ChatID == chatID {
        sendMsg(chatID, "❌ Нельзя забанить себя.")
        return
    }
    banMutex.Lock()
    targetUser.IsBanned = true
    targetUser.BanReason = banReason
    targetUser.BanExpires = time.Now().Add(time.Duration(banDurationMinutes) * time.Minute)
    banMutex.Unlock()
    saveUsers()
    sendMsg(chatID, fmt.Sprintf("✅ @%s (ID: %d) забанен на %d минут. Причина: %s", targetUser.Username, targetUser.UserID, banDurationMinutes, banReason))
    sendMsg(targetUser.ChatID, fmt.Sprintf("🚫 Вы забанены на %d минут. Причина: %s", banDurationMinutes, banReason))
    go func(user *User, durationMinutes int) {
        time.Sleep(time.Duration(durationMinutes) * time.Minute)
        banMutex.Lock()
        if user.IsBanned && time.Now().After(user.BanExpires) {
            user.IsBanned = false
            user.BanReason = ""
            user.BanExpires = time.Time{}
            saveUsers()
            banMutex.Unlock()
            sendMsg(user.ChatID, "✅ Вы разблокированы.")
            logToFile(fmt.Sprintf("@%s (ID: %d) разблокирован.", user.Username, user.UserID))
        } else {
            banMutex.Unlock()
        }
    }(targetUser, banDurationMinutes)
}

func unbanUser(user *User) {
    banMutex.Lock()
    defer banMutex.Unlock()
    user.IsBanned = false
    user.BanReason = ""
    user.BanExpires = time.Time{}
    saveUsers()
    logToFile(fmt.Sprintf("@%s (ID: %d) разблокирован.", user.Username, user.UserID))
}

// Изменение ID и ника
func processChangeIDCommand(chatID int64, text string, username string) {
    if username != AdminUser1 && username != AdminUser2 {
        sendMsg(chatID, "❌ У вас нет прав.")
        return
    }
    parts := strings.SplitN(text, " ", 3)
    if len(parts) != 3 {
        sendMsg(chatID, "❌ Формат: /change_id [ID] [новый_ID]")
        return
    }
    targetUserID, err := strconv.Atoi(parts[1])
    if err != nil {
        sendMsg(chatID, "❌ Некорректный ID.")
        return
    }
    newUserID, err := strconv.Atoi(parts[2])
    if err != nil {
        sendMsg(chatID, "❌ Некорректный новый ID.")
        return
    }
    if newUserID < MinUserID || newUserID > MaxUserID {
        sendMsg(chatID, fmt.Sprintf("❌ ID должен быть от %d до %d.", MinUserID, MaxUserID))
        return
    }
    targetUser := getUserByUserID(targetUserID)
    if targetUser == nil {
        sendMsg(chatID, fmt.Sprintf("❌ Пользователь с ID %d не найден.", targetUserID))
        return
    }
    if isIDTaken(newUserID) {
        sendMsg(chatID, "❌ Этот ID занят.")
        return
    }
    userMutex.Lock()
    targetUser.UserID = newUserID
    userMutex.Unlock()
    saveUsers()
    sendMsg(chatID, fmt.Sprintf("✅ ID @%s изменён на %d.", targetUser.Username, newUserID))
    sendMsg(targetUser.ChatID, fmt.Sprintf("✅ Ваш ID изменён на %d.", newUserID))
}

func processChangeNickCommand(chatID int64, text string, username string) {
    if username != AdminUser1 && username != AdminUser2 {
        sendMsg(chatID, "❌ У вас нет прав.")
        return
    }
    parts := strings.SplitN(text, " ", 3)
    if len(parts) != 3 {
        sendMsg(chatID, "❌ Формат: /change_nick [ID] [новый_ник]")
        return
    }
    targetUserID, err := strconv.Atoi(parts[1])
    if err != nil {
        sendMsg(chatID, "❌ Некорректный ID.")
        return
    }
    newNick := parts[2]
    targetUser := getUserByUserID(targetUserID)
    if targetUser == nil {
        sendMsg(chatID, fmt.Sprintf("❌ Пользователь с ID %d не найден.", targetUserID))
        return
    }
    if isNickTaken(newNick) {
        sendMsg(chatID, "❌ Ник занят.")
        return
    }
    userMutex.Lock()
    targetUser.MinecraftNick = newNick
    userMutex.Unlock()
    saveUsers()
    sendMsg(chatID, fmt.Sprintf("✅ Ник @%s изменён на %s.", targetUser.Username, newNick))
    sendMsg(targetUser.ChatID, fmt.Sprintf("✅ Ваш ник изменён на %s.", newNick))
}

func isIDTaken(id int) bool {
    userMutex.Lock()
    defer userMutex.Unlock()
    for _, user := range users {
        if user.UserID == id {
            return true
        }
    }
    return false
}

// Удаление вакансий
func processDeleteVacancyCommand(chatID int64, text string, username string) {
    if username != AdminUser1 && username != AdminUser2 {
        sendMsg(chatID, "❌ У вас нет прав.")
        return
    }
    parts := strings.SplitN(text, " ", 2)
    if len(parts) != 2 {
        sendMsg(chatID, "❌ Формат: /dell_sell333 [ID_вакансии]")
        return
    }
    vacancyIDToDelete, err := strconv.Atoi(parts[1])
    if err != nil {
        sendMsg(chatID, "❌ Некорректный ID.")
        return
    }
    foundIndex := -1
    for i, vac := range vacancies {
        if vac.ID == vacancyIDToDelete {
            foundIndex = i
            break
        }
    }
    if foundIndex != -1 {
        vacancies = append(vacancies[:foundIndex], vacancies[foundIndex+1:]...)
        saveVacancies()
        sendMsg(chatID, fmt.Sprintf("✅ Вакансия #%d удалена.", vacancyIDToDelete))
    } else {
        sendMsg(chatID, fmt.Sprintf("❌ Вакансия #%d не найдена.", vacancyIDToDelete))
    }
}

// Профиль
func showUserProfile(chatID int64) {
    user := getUser(chatID)
    if user == nil {
        sendMsg(chatID, "❌ Вы не зарегистрированы.")
        return
    }
    bio := user.Bio
    if bio == "" {
        bio = "Не указано"
    }
    profile := fmt.Sprintf(
        "📌 Профиль:\n🆔 ID: %d\n👤 Ник: %s\n📛 @%s\n📝 Описание: %s\n%s\n📅 Регистрация: %s",
        user.UserID, user.MinecraftNick, user.Username, bio,
        func() string {
            if user.IsBanned {
                return fmt.Sprintf("🚫 Забанен до %s\n📝 Причина: %s", user.BanExpires.Format(time.DateTime), user.BanReason)
            }
            return "✅ Активен"
        }(),
        time.Now().Format("02.01.2006"),
    )
    sendMsg(chatID, profile)
}

// Мои вакансии
func showMyVacancies(chatID int64) {
    user := getUser(chatID)
    if user == nil {
        sendMsg(chatID, "❌ Вы не зарегистрированы.")
        return
    }
    var myVacancies []Vacancy
    for _, vac := range vacancies {
        if vac.Author == user.MinecraftNick {
            myVacancies = append(myVacancies, vac)
        }
    }
    if len(myVacancies) == 0 {
        sendMsg(chatID, "ℹ️ У вас нет вакансий.")
        return
    }
    var sb strings.Builder
    sb.WriteString("📋 Ваши вакансии:\n\n")
    for _, vac := range myVacancies {
        status := "🟢 Активна"
        if vac.Accepted {
            status = fmt.Sprintf("✅ Принята: %s", vac.AcceptedBy)
        }
        paymentInfo := vac.PaymentInfo
        if paymentInfo == "" {
            paymentInfo = "Не указано"
        }
        sb.WriteString(fmt.Sprintf("#%d | %s | %s | Оплата: %s | %s\n", vac.ID, vac.Content, vac.Price, paymentInfo, status))
    }
    sendMsg(chatID, sb.String())
}

// Удаление своей вакансии
func deleteMyVacancy(chatID int64, text string) {
    user := getUser(chatID)
    if user == nil {
        sendMsg(chatID, "❌ Вы не зарегистрированы.")
        return
    }
    parts := strings.SplitN(text, " ", 2)
    if len(parts) != 2 {
        sendMsg(chatID, "❌ Формат: /delete_vacancy [ID]")
        return
    }
    vacID, err := strconv.Atoi(parts[1])
    if err != nil {
        sendMsg(chatID, "❌ Некорректный ID.")
        return
    }
    foundIndex := -1
    for i, vac := range vacancies {
        if vac.ID == vacID && vac.Author == user.MinecraftNick {
            foundIndex = i
            break
        }
    }
    if foundIndex == -1 {
        sendMsg(chatID, "❌ Вакансия не найдена или не ваша.")
        return
    }
    vacancies = append(vacancies[:foundIndex], vacancies[foundIndex+1:]...)
    saveVacancies()
    sendMsg(chatID, fmt.Sprintf("✅ Вакансия #%d удалена.", vacID))
}

// Список пользователей
func listUsers(chatID int64, username string) {
    if username != AdminUser1 && username != AdminUser2 {
        sendMsg(chatID, "❌ У вас нет прав.")
        return
    }
    if len(users) == 0 {
        sendMsg(chatID, "ℹ️ Нет пользователей.")
        logToFile("Список пользователей пуст.")
        return
    }
    var sb strings.Builder
    sb.WriteString("📋 Пользователи:\n\n")
    for _, user := range users {
        banStatus := ""
        if user.IsBanned {
            banStatus = fmt.Sprintf(" (🚫 Забанен до %s, Причина: %s)", user.BanExpires.Format(time.DateTime), user.BanReason)
        }
        bio := user.Bio
        if bio == "" {
            bio = "Не указано"
        }
        sb.WriteString(fmt.Sprintf("🆔 %d | 👤 %s | 📛 @%s | 💬 %d | 📝 %s%s\n", user.UserID, user.MinecraftNick, user.Username, user.ChatID, bio, banStatus))
    }
    logToFile(fmt.Sprintf("Отправлен список пользователей: %d записей", len(users)))
    sendMsg(chatID, sb.String())
}

// Удаление пользователя
func deleteUser(chatID int64, text string, username string) {
    if username != AdminUser1 && username != AdminUser2 {
        sendMsg(chatID, "❌ У вас нет прав.")
        return
    }
    parts := strings.SplitN(text, " ", 3)
    if len(parts) < 3 {
        sendMsg(chatID, "❌ Формат: /del_user [ID] [причина]")
        return
    }
    userID, err := strconv.Atoi(parts[1])
    if err != nil {
        sendMsg(chatID, "❌ Некорректный ID.")
        return
    }
    userMutex.Lock()
    defer userMutex.Unlock()
    for i, user := range users {
        if user.UserID == userID {
            users = append(users[:i], users[i+1:]...)
            saveUsers()
            sendMsg(chatID, fmt.Sprintf("✅ @%s (ID: %d) удалён.", user.Username, userID))
            sendMsg(user.ChatID, fmt.Sprintf("🚫 Аккаунт удалён. Причина: %s", parts[2]))
            return
        }
    }
    sendMsg(chatID, fmt.Sprintf("❌ Пользователь с ID %d не найден.", userID))
}

// Разблокировка
func unbanUserByAdmin(chatID int64, text string, username string) {
    if username != AdminUser1 && username != AdminUser2 {
        sendMsg(chatID, "❌ У вас нет прав.")
        return
    }
    parts := strings.SplitN(text, " ", 2)
    if len(parts) != 2 {
        sendMsg(chatID, "❌ Формат: /unban_user [ID]")
        return
    }
    userID, err := strconv.Atoi(parts[1])
    if err != nil {
        sendMsg(chatID, "❌ Некорректный ID.")
        return
    }
    user := getUserByUserID(userID)
    if user == nil {
        sendMsg(chatID, fmt.Sprintf("❌ Пользователь с ID %d не найден.", userID))
        return
    }
    if !user.IsBanned {
        sendMsg(chatID, fmt.Sprintf("❌ @%s (ID: %d) не заблокирован.", user.Username, userID))
        return
    }
    unbanUser(user)
    sendMsg(chatID, fmt.Sprintf("✅ @%s (ID: %d) разблокирован.", user.Username, userID))
    sendMsg(user.ChatID, "✅ Вы разблокированы.")
}

// Перезапуск
func restartBot(chatID int64, username string) {
    if username != AdminUser1 && username != AdminUser2 {
        sendMsg(chatID, "❌ У вас нет прав.")
        return
    }
    notifyAllUsers("⚠️ Бот перезагрузится через 10 секунд.")
    go func() {
        time.Sleep(10 * time.Second)
        saveUsers()
        saveVacancies()
        saveResponses()
        saveCallouts()
        logFile.Close()
        statsLogFile.Close()
        os.Exit(0)
    }()
    sendMsg(chatID, "✅ Перезапуск через 10 секунд.")
}

// Версия
func showVersion(chatID int64) {
    sendMsg(chatID, "🤖 CASSMP Bot v1.9\nДля Minecraft-сообщества.")
}
// cd и в какой папке находится код "..."
//
//запустить код go run main.go
//