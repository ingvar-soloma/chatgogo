# Product Roadmap: ChatGoGo

This document outlines the strategic roadmap for ChatGoGo, detailing the phased development from a Minimum Viable Product (MVP) to a feature-rich, monetizable platform.

## Phase 1: MVP Solidification & Core Profile Implementation

**Goal:** Launch a stable, reliable, and secure anonymous chat service. The focus is on building user trust and gathering initial feedback.

### 1.1. Advanced User Profile

-   **Objective:** Allow users to create a basic profile to improve matching quality.
-   **Features:**
    -   Implement commands (`/myprofile`, `/editprofile`) to manage profile data.
    -   Add fields to the `User` model: `Age` (int), `Gender` (string), `Interests` (`pq.StringArray`).
    -   Create a simple onboarding flow after `/start` for new users to fill in their profile.
    -   Ensure profile data can be updated at any time.

### 1.2. Enhanced Complaint System

-   **Objective:** Create a robust system for handling user complaints to ensure community safety.
-   **Features:**
    -   Implement a `/report` command that, when used in a chat, flags the partner.
    -   The `Complaint` model should capture the RoomID, ReporterID, ReportedUserID, and a reason.
    -   Develop a simple admin interface (or a set of CLI commands) to review and act on complaints (e.g., ban users).
    -   Implement a basic reputation score in the `User` model that is affected by valid complaints.

### 1.3. Content Type Handling

-   **Objective:** Gracefully handle various message types beyond plain text.
-   **Features:**
    -   Ensure the bot can receive and forward common media types like photos, GIFs, and stickers.
    -   Implement basic validation (e.g., file size limits) to prevent abuse.
    -   **User Story:** "As a user, I want to be able to send a funny GIF to my chat partner to express myself better."

## Phase 2: Advanced Matchmaking & User Engagement

**Goal:** Increase user retention and satisfaction by improving the matching algorithm and adding engaging features.

### 2.1. Profile-Based Matchmaking

-   **Objective:** Move from random matching to an intelligent algorithm based on user profiles.
-   **Implementation:**
    -   Refactor the `MatcherService` to be strategy-based. Create a `MatchingStrategy` interface.
    -   Implement a `ProfileMatchingStrategy` that prioritizes matches based on:
        -   Shared interests.
        -   Compatible age ranges.
        -   Opposite or specified gender preferences.
    -   Allow users to set basic search preferences (e.g., "match me with users interested in 'movies'").

### 2.2. User Achievements & Gamification

-   **Objective:** Encourage positive behavior and long-term engagement.
-   **Features:**
    -   Create a new `achievements` table in the database.
    -   Define and implement achievements like:
        -   **"Chatterbox":** Send 100 messages.
        -   **"Explorer":** Chat with 10 different people.
        -   **"Peacemaker":** Complete a chat without being reported.
    -   Notify users when they unlock an achievement.
    -   Display unlocked achievements on their profile (visible only to them).

### 2.3. Message Content Filtering

-   **Objective:** Proactively detect and filter harmful or inappropriate content.
-   **Implementation:**
    -   Integrate a third-party content moderation API (e.g., Google's Perspective API) or a local library for text analysis.
    -   Develop a system to automatically flag messages containing hate speech, NSFW content, etc.
    -   For media, consider integrating an image moderation service.
    -   Flagged content can be blurred, hidden, or immediately trigger a complaint.

## Phase 3: Monetization & Platform Expansion

**Goal:** Introduce sustainable revenue streams without compromising the core user experience.

### 3.1. Premium Subscription ("ChatGoGo Plus")

-   **Objective:** Offer enhanced features for paying users.
-   **Premium Features:**
    -   **Advanced Search Filters:** Allow subscribers to filter matches by specific age, gender, or multiple interests.
    -   **"Stealth Mode":** The ability to start a chat without revealing their profile details immediately.
    -   **Read Receipts:** See if a message has been read by the partner.
    -   **Ad-Free Experience:** If ads are introduced, this would be a key benefit.
-   **Implementation:** Integrate with a payment provider like Stripe or use Telegram's native payment APIs.

### 3.2. Token-Based Economy

-   **Objective:** Create a flexible system for microtransactions.
-   **Features:**
    -   Introduce a virtual currency ("Tokens").
    -   Users can earn tokens through achievements or by purchasing them.
    -   **Token Usage:**
        -   **"Rematch":** Spend tokens to try and reconnect with a previous partner (if both parties agree).
        -   **"Priority Queue":** Spend tokens to get matched faster during peak hours.
        -   **"Gifts":** Send virtual gifts (stickers, badges) to partners by spending tokens.

### 3.3. Optional Advertising

-   **Objective:** Generate revenue from non-paying users.
-   **Implementation:**
    -   Display small, unobtrusive ads in between chat sessions (e.g., while waiting for a match).
    -   Ensure ads are clearly marked and do not interrupt the chat experience itself.
    -   Premium users would have an ad-free experience.
