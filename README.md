# Telegram-Bot

This repository provides a robust architecture for a Telegram-based live support bot, designed for SaaS products or any service requiring direct user-to-manager communication. It leverages Telegram's forum topics to create organized, per-user support threads, and includes features like session management, multi-language support, and persistent storage using SQLite.

## Features

*   **Live Support Chat**: Facilitates real-time communication between users and a team of support managers.
*   **Forum Topic Integration**: Automatically creates a new topic in a designated managers' group for each user, keeping conversations neatly organized.
*   **Session Management**: Tracks the state of each support session (`waiting`, `active`, `closed`).
*   **Intelligent Message Routing**: Forwards messages from users to their dedicated manager topic and relays manager replies back to the user.
*   **Multi-Language Support**: Users can choose their preferred language (English, Russian, Ukrainian), and the bot interface adjusts accordingly.
*   **Persistent Storage**: Uses SQLite to store user information, session state, and message identifiers, ensuring data is not lost on restart.
*   **User Status Notifications**: Provides users with a status bar in their chat, informing them if they are waiting for a manager or actively in a conversation.
*   **Manager Info Cards**: Pins a message in each topic with key user details (username, language, session status) for quick reference.
*   **Handles Attachments**: Supports forwarding of text, photos, videos, documents, and other media types.

## Architecture

The project is structured to separate concerns, making it maintainable and scalable.

*   `cmd/bot/main.go`: The main application entry point that initializes and runs the bot.
*   `internal/config`: Handles loading configuration from environment variables.
*   `internal/app`: Wires together all components of the application, including the bot instance, database connection, services, and handlers.
*   `internal/handlers`: Processes incoming updates from Telegram (messages, commands, and callback queries).
*   `internal/service`: Contains the core business logic for the support system, such as managing user sessions, creating forum topics, and handling message forwarding.
*   `internal/storage`: The data access layer responsible for all database interactions with SQLite, including schema definitions and queries.

## Getting Started

Follow these instructions to get a local copy up and running.

### Prerequisites

*   Go version 1.25 or higher.
*   A Telegram Bot Token. You can get one from [@BotFather](https://t.me/BotFather).
*   A Telegram group chat with "Topics" enabled to act as the managers' chat.

### Installation

1.  **Clone the repository:**
    ```sh
    git clone https://github.com/callmemathew/Telegram-Bot.git
    cd Telegram-Bot
    ```

2.  **Configure Environment Variables:**
    Create a `.env` file in the root of the project by copying the example file:
    ```sh
    cp .env.example .env
    ```
    Now, edit the `.env` file with your specific configuration:
    ```env
    # Your Telegram Bot Token from @BotFather
    BOT_TOKEN=your_bot_token_here

    # The unique ID of your managers' forum/group chat.
    # To get the ID, you can use a bot like @userinfobot and forward a message from the group.
    MANAGERS_CHAT_ID=your_chat_id_here

    # The path where the SQLite database file will be stored.
    DB_PATH=./support.db
    ```

3.  **Install dependencies:**
    ```sh
    go mod tidy
    ```

4.  **Run the bot:**
    ```sh
    go run cmd/bot/main.go
    ```
    You should see the "Bot started" log message, and your bot will be online.

## Usage

### User Flow

1.  The user finds the bot on Telegram and sends the `/start` command.
2.  The bot presents a language selection menu (e.g., English, Russian, Ukrainian).
3.  After selecting a language, the user is prompted to send their message.
4.  When the first message is sent, a new topic is created in the managers' group, and the user sees a "waiting for manager" status message.
5.  When a manager replies, the status updates to "manager connected," and the user can chat directly.

### Manager Flow

1.  A new topic appears in the managers' group chat when a new user sends their first message.
2.  The topic is titled with the user's name or username.
3.  A pinned message at the top of the topic contains a "session card" with the user's ID, language, and current support status (`WAITING`, `ACTIVE`, `CLOSED`).
4.  Any manager can reply within the topic. The first reply activates the session, changing the status to `ACTIVE` and notifying the user.
5.  All subsequent messages from the manager in that topic are forwarded to the user.

### Commands

The following commands are available to users in their private chat with the bot:

*   `/start`: Initializes the conversation. Prompts for language selection if not already set.
*   `/lang`: Allows the user to change their preferred language at any time.
*   `/stop`: Informs the user that the session is stopped. Messages will create a new session.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
