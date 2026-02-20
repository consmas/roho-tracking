class StreamSessionStatusPollerJob
  include Sidekiq::Job

  sidekiq_options queue: :streams, retry: 10, backtrace: false

  def perform
    loop do
      result = StreamSessionStatusUpdater.call
      Rails.logger.info(
        {
          event: "stream_status_poll",
          checked: result.checked,
          activated: result.activated,
          failed: result.failed
        }.to_json
      )

      sleep(poll_interval_seconds)
    end
  rescue Sidekiq::Shutdown
    Rails.logger.info({ event: "stream_status_poller_shutdown" }.to_json)
  rescue StandardError => e
    Rails.logger.error({ event: "stream_status_poller_failed", error: e.message }.to_json)
    sleep(2)
    retry
  end

  private

  def poll_interval_seconds
    ENV.fetch("STREAM_STATUS_POLL_INTERVAL_SECONDS", "5").to_i
  end
end
