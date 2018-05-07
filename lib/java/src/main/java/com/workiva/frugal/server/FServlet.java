package com.workiva.frugal.server;

import com.workiva.frugal.processor.FProcessor;
import com.workiva.frugal.protocol.FProtocolFactory;
import com.workiva.frugal.transport.TMemoryOutputBuffer;
import org.apache.thrift.TException;
import org.apache.thrift.transport.TMemoryInputTransport;
import org.apache.thrift.transport.TTransport;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import javax.servlet.ServletException;
import javax.servlet.http.HttpServlet;
import javax.servlet.http.HttpServletRequest;
import javax.servlet.http.HttpServletResponse;

import java.io.DataInputStream;
import java.io.EOFException;
import java.io.IOException;
import java.io.InputStream;
import java.io.OutputStream;
import java.util.Base64;

/**
 * Processes POST requests as Frugal requests for a processor.
 * <p>
 * By default, the HTTP request is limited to a 64MB Frugal payload size to
 * prevent client requests from causing the server to allocate too much memory.
 * <p>
 * The HTTP request may include an X-Frugal-Payload-Limit header setting the size
 * limit of responses from the server.
 * <p>
 * The HTTP processor returns a 500 response for any runtime errors when executing
 * a frame, a 400 response for an invalid frame, and a 413 response if the response
 * exceeds the payload limit specified by the client.
 * <p>
 * Both the request and response are base64 encoded.
 */
@SuppressWarnings("serial")
public class FServlet extends HttpServlet {
    private static final Logger LOGGER = LoggerFactory.getLogger(FServlet.class);

    private static final int DEFAULT_MAX_REQUEST_SIZE = 64 * 1024 * 1024;

    private final FProcessor processor;
    private final FProtocolFactory inProtocolFactory;
    private final FProtocolFactory outProtocolFactory;
    private final int maxRequestSize;

    /**
     * Creates a servlet for the specified processor and protocol factory, which
     * is used for both input and output.
     */
    public FServlet(FProcessor processor, FProtocolFactory protocolFactory) {
        this(processor, protocolFactory, DEFAULT_MAX_REQUEST_SIZE);
    }

    /**
     * Creates a servlet for the specified processor and protocol factory, which
     * is used for both input and output.
     *
     * @param maxRequestSize the maximum Frugal request size in bytes
     */
    public FServlet(FProcessor processor, FProtocolFactory protocolFactory, int maxRequestSize) {
        this(processor, protocolFactory, protocolFactory, maxRequestSize);
    }

    /**
     * Creates a servlet for the specified processor and input/output protocol
     * factories.
     */
    public FServlet(FProcessor processor, FProtocolFactory inProtocolFactory, FProtocolFactory outProtocolFactory) {
        this(processor, inProtocolFactory, outProtocolFactory, DEFAULT_MAX_REQUEST_SIZE);
    }

    /**
     * Creates a servlet for the specified processor and input/output protocol
     * factories.
     *
     * @param maxRequestSize the maximum Frugal request size in bytes
     */
    public FServlet(
            FProcessor processor,
            FProtocolFactory inProtocolFactory,
            FProtocolFactory outProtocolFactory,
            int maxRequestSize) {
        this.processor = processor;
        this.inProtocolFactory = inProtocolFactory;
        this.outProtocolFactory = outProtocolFactory;
        this.maxRequestSize = maxRequestSize;
    }

    @Override
    public void doPost(HttpServletRequest req, HttpServletResponse resp) throws ServletException, IOException {
        byte[] frame;
        try (InputStream decoderIn = Base64.getDecoder().wrap(req.getInputStream());
                DataInputStream dataIn = new DataInputStream(decoderIn)) {
            try {
                long size = dataIn.readInt() & 0xffff_ffffL;
                if (size > maxRequestSize) {
                    LOGGER.debug("Request size too large. Received: {}, Limit: {}", size, maxRequestSize);
                    resp.setStatus(HttpServletResponse.SC_REQUEST_ENTITY_TOO_LARGE);
                    return;
                }

                frame = new byte[(int) size];
                dataIn.readFully(frame);
            } catch (EOFException e) {
                LOGGER.debug("Request body too short");
                resp.setStatus(HttpServletResponse.SC_BAD_REQUEST);
                return;
            }

            if (dataIn.read() != -1) {
                LOGGER.debug("Request body too long");
                resp.setStatus(HttpServletResponse.SC_BAD_REQUEST);
                return;
            }
        }

        TTransport inTransport = new TMemoryInputTransport(frame);
        TMemoryOutputBuffer outTransport = new TMemoryOutputBuffer();
        try {
            processor.process(inProtocolFactory.getProtocol(inTransport), outProtocolFactory.getProtocol(outTransport));
        } catch (RuntimeException e) {
            // Already logged by FBaseProcessor.
            resp.setStatus(HttpServletResponse.SC_INTERNAL_SERVER_ERROR);
            return;
        } catch (TException e) {
            LOGGER.error("Frugal processor returned unhandled error", e);
            resp.setStatus(HttpServletResponse.SC_INTERNAL_SERVER_ERROR);
            return;
        }

        byte[] data = outTransport.getWriteBytes();

        int responseLimit = getResponseLimit(req);
        if (responseLimit > 0 && outTransport.size() > responseLimit) {
            LOGGER.debug("Response size too large for client. Received: {}, Limit: {}",
                    outTransport.size(), responseLimit);
            resp.setStatus(HttpServletResponse.SC_REQUEST_ENTITY_TOO_LARGE);
            return;
        }

        resp.setContentType("application/x-frugal");
        resp.setHeader("Content-Transfer-Encoding", "base64");
        try (OutputStream out = Base64.getEncoder().wrap(resp.getOutputStream())) {
            out.write(data);
        }
    }

    // Visible for testing.
    static int getResponseLimit(HttpServletRequest req) {
        String payloadHeader = req.getHeader("x-frugal-payload-limit");
        int responseLimit;
        try {
            responseLimit = Integer.parseInt(payloadHeader);
        } catch (NumberFormatException ignored) {
            responseLimit = 0;
        }
        return responseLimit;
    }
}
