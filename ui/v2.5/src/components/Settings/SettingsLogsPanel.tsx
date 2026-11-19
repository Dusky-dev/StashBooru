import React, { useEffect, useLayoutEffect, useRef, useState } from "react";
import { FormattedMessage } from "react-intl";
import * as GQL from "src/core/generated-graphql";
import { useLoggingSubscribe, queryLogs } from "src/core/StashService";
import { SelectSetting } from "./Inputs";
import { SettingSection } from "./SettingSection";

function convertTime(logEntry: GQL.LogEntryDataFragment) {
  function pad(val: number) {
    let ret = val.toString();
    if (val <= 9) {
      ret = `0${ret}`;
    }

    return ret;
  }

  const date = new Date(logEntry.time);
  const month = date.getMonth() + 1;
  const day = date.getDate();
  let dateStr = `${date.getFullYear()}-${pad(month)}-${pad(day)}`;
  dateStr += ` ${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(
    date.getSeconds()
  )}`;

  return dateStr;
}

function levelClass(level: string) {
  return level.toLowerCase().trim();
}

const levelLabels: Record<string, string> = {
  Trace: "TRACE",
  Debug: "DEBUG",
  Info: "INFO",
  Warning: "WARN",
  Error: "ERROR",
  Progress: "PROG",
};

interface ILogElementProps {
  logEntry: LogEntry;
}

const LogElement: React.FC<ILogElementProps> = ({ logEntry }) => {
  const level = (levelLabels[logEntry.level] ?? logEntry.level).padEnd(5);

  return (
    <div className={`log-line log-line-${levelClass(logEntry.level)}`}>
      <span className="log-time" title={logEntry.time}>
        {logEntry.time.slice(11)}
      </span>
      <span className={`log-level ${levelClass(logEntry.level)}`}>{level}</span>
      <span className="log-message">{logEntry.message}</span>
    </div>
  );
};

class LogEntry {
  public time: string;
  public level: string;
  public message: string;
  public id: string;

  private static nextId: number = 0;

  public constructor(logEntry: GQL.LogEntryDataFragment) {
    this.time = convertTime(logEntry);
    this.level = logEntry.level;
    this.message = logEntry.message;

    const id = LogEntry.nextId++;
    this.id = id.toString();
  }
}

// maximum number of log entries to keep - entries are discarded oldest-first
const MAX_LOG_ENTRIES = 50000;
// maximum number of log entries to display
const MAX_DISPLAY_LOG_ENTRIES = 1000;
const logLevels = ["Trace", "Debug", "Info", "Warning", "Error"];

export const SettingsLogsPanel: React.FC = () => {
  const [entries, setEntries] = useState<LogEntry[]>([]);
  const { data, error } = useLoggingSubscribe();
  const [logLevel, setLogLevel] = useState<string>("Info");

  useEffect(() => {
    async function getInitialLogs() {
      const logQuery = await queryLogs();
      if (logQuery.error) return;

      const initEntries = logQuery.data.logs.map((e) => new LogEntry(e));
      if (initEntries.length !== 0) {
        setEntries((prev) => {
          return [...prev, ...initEntries].slice(0, MAX_LOG_ENTRIES);
        });
      }
    }

    // About shares this page, so a failed log read must not break it.
    getInitialLogs().catch(() => undefined);
  }, []);

  useEffect(() => {
    if (!data?.loggingSubscribe) return;

    const newEntries = data.loggingSubscribe.map((e) => new LogEntry(e));
    newEntries.reverse();
    setEntries((prev) => {
      return [...newEntries, ...prev].slice(0, MAX_LOG_ENTRIES);
    });
  }, [data]);

  // Newest entries are kept first; a terminal prints them at the bottom.
  const displayEntries = entries
    .filter(filterByLogLevel)
    .slice(0, MAX_DISPLAY_LOG_ENTRIES)
    .reverse();

  const screen = useRef<HTMLDivElement>(null);
  const [following, setFollowing] = useState(true);
  const latestID = displayEntries[displayEntries.length - 1]?.id;

  useLayoutEffect(() => {
    const element = screen.current;
    if (element && following && latestID !== undefined)
      element.scrollTop = element.scrollHeight;
  }, [latestID, following]);

  function onScroll() {
    const element = screen.current;
    if (!element) return;
    const atBottom =
      element.scrollHeight - element.scrollTop - element.clientHeight < 24;
    if (atBottom !== following) setFollowing(atBottom);
  }

  function maybeRenderError() {
    if (error) {
      return (
        <div className="error">
          Error connecting to log server: {error.message}
        </div>
      );
    }
  }

  function filterByLogLevel(logEntry: LogEntry) {
    if (logLevel === "Trace") return true;

    const logLevelIndex = logLevels.indexOf(logLevel);
    const levelIndex = logLevels.indexOf(logEntry.level);

    return levelIndex >= logLevelIndex;
  }

  return (
    <div id="about-logs">
      <SettingSection headingID="config.categories.logs">
        <SelectSetting
          id="log-level"
          headingID="config.logs.log_level"
          value={logLevel}
          onChange={(v) => setLogLevel(v)}
        >
          {logLevels.map((level) => (
            <option key={level} value={level}>
              {level}
            </option>
          ))}
        </SelectSetting>
      </SettingSection>

      {/* A fixed-height box keeps About and the Changelog within reach. */}
      <div className="log-terminal">
        <div className="log-terminal-bar">
          <span className="log-terminal-title">
            <FormattedMessage id="config.logs.console_title" />
          </span>
          <span
            className={`log-terminal-status ${error ? "offline" : "online"}`}
          >
            <FormattedMessage
              id={error ? "config.logs.disconnected" : "config.logs.live"}
            />
          </span>
          <span className="log-terminal-count">
            <FormattedMessage
              id="config.logs.line_count"
              values={{ count: displayEntries.length }}
            />
          </span>
        </div>
        <div
          className="logs logs-compact"
          ref={screen}
          onScroll={onScroll}
          role="log"
          aria-live="off"
          tabIndex={0}
        >
          {maybeRenderError()}
          {displayEntries.length === 0 && !error && (
            <div className="log-empty">
              <FormattedMessage id="config.logs.empty" />
            </div>
          )}
          {displayEntries.map((logEntry) => (
            <LogElement logEntry={logEntry} key={logEntry.id} />
          ))}
          <div className="log-prompt" aria-hidden="true">
            <span className="log-prompt-user">stash@booru</span>
            <span className="log-prompt-path">:~$</span> tail -f stash.log
            --level={logLevel.toLowerCase()}
            <span className="log-cursor" />
          </div>
        </div>
        {!following && (
          <button
            type="button"
            className="log-terminal-jump"
            onClick={() => setFollowing(true)}
          >
            <FormattedMessage id="config.logs.jump_to_latest" />
          </button>
        )}
      </div>
    </div>
  );
};
