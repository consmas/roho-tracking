class StreamSessionStatusUpdater
  Result = Struct.new(:checked, :activated, :failed, keyword_init: true)

  def self.call(limit: ENV.fetch("STREAM_STATUS_POLL_LIMIT", "200").to_i)
    new(limit:).call
  end

  def initialize(limit:, client: MediaMtxClient.new)
    @limit = limit
    @client = client
  end

  def call
    sessions = StreamSession.requested.order(created_at: :asc).limit(limit).to_a
    return Result.new(checked: 0, activated: 0, failed: 0) if sessions.empty?

    paths = client.paths_by_name
    activated = 0
    failed = 0
    now = Time.current

    sessions.each do |session|
      path_info = paths[session.stream_name]
      if path_ready?(path_info)
        session.update!(status: :active, last_error: nil)
        activated += 1
      elsif timed_out?(session, now)
        session.update!(status: :failed, ended_at: Time.current, last_error: "stream_not_ready_timeout")
        failed += 1
      end
    end

    Result.new(checked: sessions.size, activated:, failed:)
  end

  private

  attr_reader :limit, :client

  def request_timeout_seconds
    ENV.fetch("STREAM_REQUEST_TIMEOUT_SECONDS", "90").to_i
  end

  def timed_out?(session, now)
    deadline = session.activation_deadline_at || (session.created_at + request_timeout_seconds.seconds)
    deadline <= now
  end

  def path_ready?(path_info)
    return false unless path_info.is_a?(Hash)

    readiness = fetch_boolean(path_info, "ready", "sourceReady", "isReady")
    return readiness unless readiness.nil?

    readers = fetch_integer(path_info, "readers")
    return true if readers && readers > 0

    bytes = fetch_integer(path_info, "bytesReceived")
    return true if bytes && bytes > 0

    false
  end

  def fetch_boolean(hash, *keys)
    keys.each do |key|
      next unless hash.key?(key)

      value = hash[key]
      return value if value == true || value == false
    end
    nil
  end

  def fetch_integer(hash, key)
    value = hash[key]
    return value if value.is_a?(Integer)
    return value.to_i if value.is_a?(String) && value.match?(/\A\d+\z/)

    nil
  end
end
